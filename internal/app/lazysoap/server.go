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
		metrics: rest.NewMetrics(),
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
		logger.Info(ctx, "Closing server (context done)")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := srv.Shutdown(ctx)
		if err != nil {
			logger.Error(ctx, "Failed to shutdown server", "err", err)
		}
	}()

	logger.Info(ctx, "Start to listen http requests", "address", s.address)
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
