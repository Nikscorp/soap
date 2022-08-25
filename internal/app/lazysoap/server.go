package lazysoap

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

const (
	ioTimeout   = 15 * time.Second
	readTimeout = 15 * time.Second
	idleTimeout = 15 * time.Second
)

type Server struct {
	Address string
	TVMeta  *tvmeta.Client
}

func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.Address,
		WriteTimeout:      ioTimeout,
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
	r.Use(handlers.CORS(handlers.AllowedOrigins([]string{"*"})))
	r.Use(handlers.RecoveryHandler())
	r.Use(func(next http.Handler) http.Handler { return handlers.LoggingHandler(log.Writer(), next) })

	r.Handle("/id/{id}", http.HandlerFunc(s.idHandler)).Methods("GET", "POST")
	r.Handle("/search/{query}", http.HandlerFunc(s.searchHandler)).Methods("GET", "POST")
	rest.AddFileServer(r)

	return r
}
