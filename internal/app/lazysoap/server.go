// Package classification LazySoap.
//
// Schemes: https
// Version: OVERRIDE_VERSION
// License: MIT http://opensource.org/licenses/MIT
// Contact: Nikita Voynov<voynov@nikscorp.com> https://www.nikscorp.com
//
// Consumes:
// - application/json
//
// Produces:
// - application/json
//
// swagger:meta
package lazysoap

import (
	"context"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/Nikscorp/soap/internal/pkg/trace"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	tracerName = "github.com/Nikscorp/internal/app/lazysoap"
)

type Config struct {
	Address           string          `yaml:"listen_addr" env:"LAZYSOAP_LISTEN_ADDR" env-default:"0.0.0.0:8080"`
	ReadTimeout       time.Duration   `yaml:"read_timeout" env:"LAZYSOAP_READ_TIMEOUT" env-default:"10s"`
	ReadHeaderTimeout time.Duration   `yaml:"read_header_timeout" env:"LAZYSOAP_READ_HEADER_TIMEOUT" env-default:"10s"`
	WriteTimeout      time.Duration   `yaml:"write_timeout" env:"LAZYSOAP_WRITE_TIMEOUT" env-default:"10s"`
	IdleTimeout       time.Duration   `yaml:"idle_timeout" env:"LAZYSOAP_IDLE_TIMEOUT" env-default:"10s"`
	GracefulTimeout   time.Duration   `yaml:"graceful_timeout" env:"LAZYSOAP_GRACEFUL_TIMEOUT" env-default:"10s"`
	ImgClient         ImgClientConfig `yaml:"img_client"`
}

type ImgClientConfig struct {
	Timeout         time.Duration `yaml:"timeout" env:"LAZYSOAP_IMG_CLIENT_TIMEOUT" env-default:"5s"`
	MaxIdleConns    int           `yaml:"max_idle_conns" env:"LAZYSOAP_IMG_CLIENT_MAX_IDLE_CONNS" env-default:"100"`
	IdleConnTimeout time.Duration `yaml:"idle_conn_timeout" env:"LAZYSOAP_IMG_CLIENT_IDLE_CONN_TIMEOUT" env-default:"60s"`
}

type Server struct {
	config    Config
	tvMeta    tvMetaClient
	metrics   *rest.Metrics
	imgClient *http.Client
}

type tvMetaClient interface {
	SearchTVShows(ctx context.Context, query string) (*tvmeta.TVShows, error)
	TVShowAllSeasonsWithDetails(ctx context.Context, id int, language string) (*tvmeta.AllSeasonsWithDetails, error)
}

func New(config Config, tvMetaClient tvMetaClient) *Server {
	return &Server{
		config:  config,
		tvMeta:  tvMetaClient,
		metrics: rest.NewMetrics(),
		imgClient: &http.Client{
			Timeout: config.ImgClient.Timeout,
			Transport: otelhttp.NewTransport(&http.Transport{
				MaxIdleConns:    config.ImgClient.MaxIdleConns,
				IdleConnTimeout: config.ImgClient.IdleConnTimeout,
			}),
		},
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

	go func() {
		<-ctx.Done()
		logger.Info(ctx, "Closing server (context done)")
		ctx, cancel := context.WithTimeout(context.Background(), s.config.GracefulTimeout)
		defer cancel()
		err := srv.Shutdown(ctx)
		if err != nil {
			logger.Error(ctx, "Failed to shutdown server", "err", err)
		}
	}()

	logger.Info(ctx, "Start to listen http requests", "address", s.config.Address)
	return srv.ListenAndServe()
}

func (s *Server) newRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(rest.LogRequest)

	r.Use(middleware.Recoverer)
	r.Use(cors.AllowAll().Handler)
	r.Use(s.metrics.Middleware)
	r.Use(rest.Ping)
	r.Use(rest.Version(trace.Version))
	r.Use(rest.TraceIDToOutHeader)

	r.HandleFunc("/id/{id}", s.idHandler)
	r.HandleFunc("/search/{query}", s.searchHandler)
	r.HandleFunc("/img/{path}", s.imgProxyHandler)

	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/*", pprof.Index)

	rest.AddFileServer(r)

	return otelhttp.NewHandler(r, "lazysoap.http.server", otelhttp.WithFilter(func(r *http.Request) bool {
		return strings.HasPrefix(r.URL.Path, "/id/") ||
			strings.HasPrefix(r.URL.Path, "/search/") ||
			strings.HasPrefix(r.URL.Path, "/img/")
	}))
}
