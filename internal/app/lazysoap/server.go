// Package lazysoap implements the LazySoap HTTP server: it exposes the
// JSON API for searching TV series and surfacing their best episodes,
// proxies TMDB poster images, and serves the SPA bundle.
package lazysoap

import (
	"context"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
	Address                       string          `env:"LAZYSOAP_LISTEN_ADDR"                      env-default:"0.0.0.0:8080"                                        yaml:"listen_addr"`
	ReadTimeout                   time.Duration   `env:"LAZYSOAP_READ_TIMEOUT"                     env-default:"10s"                                                 yaml:"read_timeout"`
	ReadHeaderTimeout             time.Duration   `env:"LAZYSOAP_READ_HEADER_TIMEOUT"              env-default:"10s"                                                 yaml:"read_header_timeout"`
	WriteTimeout                  time.Duration   `env:"LAZYSOAP_WRITE_TIMEOUT"                    env-default:"10s"                                                 yaml:"write_timeout"`
	IdleTimeout                   time.Duration   `env:"LAZYSOAP_IDLE_TIMEOUT"                     env-default:"10s"                                                 yaml:"idle_timeout"`
	GracefulTimeout               time.Duration   `env:"LAZYSOAP_GRACEFUL_TIMEOUT"                 env-default:"10s"                                                 yaml:"graceful_timeout"`
	DefaultBestQuantile           float32         `env:"LAZYSOAP_DEFAULT_BEST_QUANTILE"            env-default:"0.9"                                                 yaml:"default_best_quantile"`
	DefaultBestMinEpisodes        int             `env:"LAZYSOAP_DEFAULT_BEST_MIN_EPISODES"        env-default:"3"                                                   yaml:"default_best_min_episodes"`
	FeaturedCount                 int             `env:"LAZYSOAP_FEATURED_COUNT"                   env-default:"3"                                                   yaml:"featured_count"`
	FeaturedMinVoteCount          int             `env:"LAZYSOAP_FEATURED_MIN_VOTE_COUNT"          env-default:"100"                                                 yaml:"featured_min_vote_count"`
	FeaturedExtraIDs              []int           `env:"LAZYSOAP_FEATURED_EXTRA_IDS"               env-default:"1399,1396,1668,2316,1418,66732,1100,42009,1622,4607" env-separator:","                       yaml:"featured_extra_ids"`
	FeaturedExtrasRefreshInterval time.Duration   `env:"LAZYSOAP_FEATURED_EXTRAS_REFRESH_INTERVAL" env-default:"24h"                                                 yaml:"featured_extras_refresh_interval"`
	RatingsSource                 string          `env:"LAZYSOAP_RATINGS_SOURCE"                   env-default:"tmdb"                                                yaml:"ratings_source"`
	ImgClient                     ImgClientConfig `yaml:"img_client"`
	ImgCache                      ImgCacheConfig  `yaml:"img_cache"`
}

type ImgClientConfig struct {
	Timeout         time.Duration `env:"LAZYSOAP_IMG_CLIENT_TIMEOUT"           env-default:"5s"  yaml:"timeout"`
	MaxIdleConns    int           `env:"LAZYSOAP_IMG_CLIENT_MAX_IDLE_CONNS"    env-default:"100" yaml:"max_idle_conns"`
	IdleConnTimeout time.Duration `env:"LAZYSOAP_IMG_CLIENT_IDLE_CONN_TIMEOUT" env-default:"60s" yaml:"idle_conn_timeout"`
}

// ImgCacheConfig configures the in-process bytes cache for /img. A zero value
// (size <= 0 or ttl <= 0) leaves the cache in pass-through mode — every
// request hits TMDB. BrowserMaxAge controls only the Cache-Control header
// emitted on /img responses; it is independent of the server-side TTL because
// TMDB poster paths are content-addressed in practice (a long browser cache
// is safe even if the server-side entry has rotated out).
type ImgCacheConfig struct {
	Size          int           `env:"LAZYSOAP_IMG_CACHE_SIZE"      env-default:"512"    yaml:"size"`
	TTL           time.Duration `env:"LAZYSOAP_IMG_CACHE_TTL"       env-default:"168h"   yaml:"ttl"`
	BrowserMaxAge time.Duration `env:"LAZYSOAP_IMG_BROWSER_MAX_AGE" env-default:"86400s" yaml:"browser_max_age"`
}

type Server struct {
	config         Config
	tvMeta         tvMetaClient
	metrics        *rest.Metrics
	imgClient      *http.Client
	imgCache       *imgCache
	version        string
	featuredExtras *featuredExtrasCache
}

type tvMetaClient interface {
	SearchTVShows(ctx context.Context, query string) (*tvmeta.TVShows, error)
	TVShowAllSeasonsWithDetails(ctx context.Context, id int, language string) (*tvmeta.AllSeasonsWithDetails, error)
	PopularTVShows(ctx context.Context, language string) ([]*tvmeta.TVShow, error)
	TVShowDetails(ctx context.Context, id int, language string) (*tvmeta.TvShowDetails, error)
}

// New constructs the Server. cache may be nil — a nil *imgCache behaves as
// the pass-through sentinel (every /img request goes upstream), which is the
// shape unit tests use to keep mock-roundtripper call counts deterministic.
func New(config Config, tvMetaClient tvMetaClient, cache *imgCache, version string) *Server {
	return &Server{
		config:  config,
		tvMeta:  tvMetaClient,
		metrics: rest.NewMetrics(),
		imgClient: &http.Client{
			Timeout: config.ImgClient.Timeout,
			Transport: func() http.RoundTripper {
				baseTransport, ok := http.DefaultTransport.(*http.Transport)
				if !ok {
					panic("http.DefaultTransport is not *http.Transport")
				}
				clone := baseTransport.Clone()
				clone.MaxIdleConns = config.ImgClient.MaxIdleConns
				clone.MaxIdleConnsPerHost = config.ImgClient.MaxIdleConns
				clone.IdleConnTimeout = config.ImgClient.IdleConnTimeout
				return clone
			}(),
		},
		imgCache:       cache,
		version:        version,
		featuredExtras: newFeaturedExtrasCache(),
	}
}

func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.config.Address,
		WriteTimeout:      s.config.WriteTimeout,
		ReadHeaderTimeout: s.config.ReadHeaderTimeout,
		ReadTimeout:       s.config.ReadTimeout,
		IdleTimeout:       s.config.IdleTimeout,
		Handler:           s.newRouter(),
	}

	// Warm and periodically refresh the featured-extras cache so the request
	// path doesn't have to round-trip TMDB for static curated IDs. Async on
	// purpose: a slow / down TMDB at boot must not block the server from
	// listening (k8s liveness, fast restarts).
	go s.runFeaturedExtrasRefresh(ctx)

	//nolint:gosec // request-scoped ctx is already cancelled here; we deliberately use a fresh one for graceful shutdown
	go func() {
		<-ctx.Done()
		logger.Info(ctx, "Closing server (context done)")
		ctx, cancel := context.WithTimeout(context.Background(), s.config.GracefulTimeout)
		defer cancel()
		//nolint:contextcheck
		err := srv.Shutdown(ctx)
		if err != nil {
			//nolint:contextcheck
			logger.Error(ctx, "Failed to shutdown server", "err", err)
		}
	}()

	logger.Info(ctx, "Start to listen http requests", "address", s.config.Address)
	return srv.ListenAndServe()
}

func (s *Server) newRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(
		middleware.RequestID,
		rest.RequestIDHeader,
		rest.LogRequest,
		middleware.Recoverer,
		cors.AllowAll().Handler,
		s.metrics.Middleware,
		rest.Ping,
		rest.Version(s.version),
	)

	r.HandleFunc("/id/{id}", s.idHandler)
	r.HandleFunc("/search/{query}", s.searchHandler)
	r.HandleFunc("/img/{path}", s.imgProxyHandler)
	r.HandleFunc("/featured", s.featuredHandler)
	r.HandleFunc("/meta", s.metaHandler)

	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/*", pprof.Index)

	rest.AddFileServer(r)

	return r
}
