package lazysoap

import (
	"context"
	"log"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	writeTimeout = 60 * time.Second
	readTimeout  = 15 * time.Second
	idleTimeout  = 15 * time.Second
)

type Server struct {
	address   string
	tvMeta    tvMetaClient
	metrics   *rest.Metrics
	imgClient *http.Client
}

type tvMetaClient interface {
	SearchTVShows(ctx context.Context, query string) (*tvmeta.TVShows, error)
	TVShowAllSeasonsWithDetails(ctx context.Context, id int, language string) (*tvmeta.AllSeasonsWithDetails, error)
}

func New(address string, tvMetaClient tvMetaClient) *Server {
	return &Server{
		address: address,
		tvMeta:  tvMetaClient,
		metrics: rest.NewMetrics([]string{"id", "search", "img"}),
		imgClient: &http.Client{
			Timeout: time.Second * 5,
			Transport: &http.Transport{
				MaxIdleConns:    100,
				IdleConnTimeout: 60 * time.Second,
			},
		},
	}
}

func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.address,
		WriteTimeout:      writeTimeout,
		ReadHeaderTimeout: readTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
		Handler:           s.newRouter(),
	}

	go func() {
		<-ctx.Done()
		log.Printf("[INFO] Closing server (context done)")
		err := srv.Close()
		if err != nil {
			log.Printf("[ERROR] Failed to close server: %v", err)
		}
	}()

	log.Printf("[INFO] Start to listen http requests")
	return srv.ListenAndServe()
}

func (s *Server) newRouter() *mux.Router {
	r := mux.NewRouter()
	r.Use(s.metrics.Middleware)
	r.Use(handlers.CORS(handlers.AllowedOrigins([]string{"*"})))
	r.Use(handlers.RecoveryHandler())
	r.Use(func(next http.Handler) http.Handler { return handlers.LoggingHandler(log.Writer(), next) })
	r.Use(rest.Ping)

	r.HandleFunc("/id/{id}", s.idHandler).Methods("GET", "POST")
	r.HandleFunc("/search/{query}", s.searchHandler).Methods("GET", "POST")
	r.HandleFunc("/img/{path}", s.imgProxyHandler).Methods("GET")

	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/debug/pprof", pprof.Index)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	rest.AddFileServer(r)

	return r
}
