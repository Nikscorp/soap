package lazysoap

import (
	"io"
	"net/http"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/go-chi/chi/v5"
)

func (s *Server) imgProxyHandler(w http.ResponseWriter, r *http.Request) {
	path := chi.URLParam(r, "path")
	size := r.URL.Query().Get("size")
	ctx := logger.WithAttrs(r.Context(), "path", path, "size", size)
	url := tvmeta.GetURLByPosterPathWithSize(path, size)

	//nolint:gosec // url is built from a TMDB poster path; this handler intentionally proxies that fixed host
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logger.Error(ctx, "Failed to create img-proxy request", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	//nolint:gosec // see above
	resp, err := s.imgClient.Do(req)
	if err != nil {
		logger.Error(ctx, "Failed to perform img-proxy request", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer func() { _ = resp.Body.Close() }()

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
