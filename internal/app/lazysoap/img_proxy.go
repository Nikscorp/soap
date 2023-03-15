package lazysoap

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func (s *Server) imgProxyHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer(tracerName).Start(r.Context(), "server.idHandler")
	defer span.End()

	path := chi.URLParam(r, "path")
	span.SetAttributes(attribute.String("path", path))
	url := tvmeta.GetURLByPosterPath(path)

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create img-proxy request %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	resp, err := s.imgClient.Do(req)
	if err != nil {
		log.Printf("[ERROR] Failed to perform img-proxy request %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		return
	}

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("[ERROR] Failed to write img-proxy resp %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}