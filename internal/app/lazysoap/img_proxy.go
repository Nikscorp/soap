package lazysoap

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
)

func (s *Server) imgProxyHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer(tracerName).Start(r.Context(), "server.imgProxyHandler")
	defer span.End()

	path := chi.URLParam(r, "path")
	ctx = logger.ContextWithAttrs(ctx, "path", path)
	url := tvmeta.GetURLByPosterPath(path)

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logger.Error(ctx, "Failed to create img-proxy request", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	resp, err := s.imgClient.Do(req)
	if err != nil {
		logger.Error(ctx, "Failed to perform img-proxy request", "err", err)
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
		logger.Error(ctx, "Failed to write img-proxy resp", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
