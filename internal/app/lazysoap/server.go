// Package classification LazySoap
//
// Schemes: http, https
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
	"log"
	"net/http"
	"net/http/pprof"
	"time"

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
	writeTimeout = 60 * time.Second
	readTimeout  = 15 * time.Second
	idleTimeout  = 15 * time.Second
	tracerName   = "github.com/Nikscorp/internal/app/lazysoap"
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
		metrics: rest.NewMetrics([]string{"id", "search", "img", "ping"}),
		imgClient: &http.Client{
			Timeout: time.Second * 5,
			Transport: otelhttp.NewTransport(&http.Transport{
				MaxIdleConns:    100,
				IdleConnTimeout: 60 * time.Second,
			}),
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

func (s *Server) newRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: log.Default(), NoColor: true}))

	r.Use(middleware.Recoverer)
	r.Use(cors.AllowAll().Handler)
	r.Use(s.metrics.Middleware)
	r.Use(rest.Ping)
	r.Use(rest.Version(trace.Version))

	r.Handle("/id/{id}", otelhttp.NewHandler(http.HandlerFunc(s.idHandler), "id"))
	r.Handle("/search/{query}", otelhttp.NewHandler(http.HandlerFunc(s.searchHandler), "search"))
	r.Handle("/img/{path}", otelhttp.NewHandler(http.HandlerFunc(s.imgProxyHandler), "img"))

	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/*", pprof.Index)

	rest.AddFileServer(r)

	return r
}
