package lazysoap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/go-chi/chi/v5"
)

// maxImgBytes caps a single cached poster body. w780 is the largest allowed
// size and tops out near 250 KB on TMDB; 2 MiB leaves a wide safety margin
// without exposing the cache to a poisoned upstream that streams forever.
const maxImgBytes int64 = 2 * 1024 * 1024

// errUpstreamNotFound and errUpstreamFailed are the two outcomes the cache
// fetch closure can surface. Sentinel'd so the handler can map upstream-404
// → 404 without leaking a string-match against the wrapped %w. Anything else
// (transport failures, oversized bodies, non-image content-types, non-200
// non-404 statuses) flows through errUpstreamFailed → 502.
var (
	errUpstreamNotFound = errors.New("img upstream returned 404")
	errUpstreamFailed   = errors.New("img upstream failed")
)

func (s *Server) imgProxyHandler(w http.ResponseWriter, r *http.Request) {
	path := chi.URLParam(r, "path")
	rawSize := r.URL.Query().Get("size")
	size := tvmeta.NormalizePosterSize(rawSize)
	ctx := logger.WithAttrs(r.Context(), "path", path, "size", size)

	key := imgCacheKey(path, size)

	// Featured posters are pinned in featuredImgs and served without ever
	// touching imgCache (the LRU). This guarantees that prewarmed featured
	// entries are never evicted by general /id/{id} traffic — see
	// featuredImgCache for the rationale and trade-offs.
	entry, ok := s.featuredImgs.lookup(key)
	if !ok {
		var err error
		entry, err = s.imgCache.GetOrFetch(ctx, key, func(ctx context.Context) (ImgCacheEntry, error) {
			return s.fetchPoster(ctx, path, size)
		})
		if err != nil {
			if errors.Is(err, errUpstreamNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			// Caller-side cancellation arrives via ctx.Err() from
			// lrucache.GetOrFetch; the request is already gone, so a 502
			// status is harmless (client will not see it).
			w.WriteHeader(http.StatusBadGateway)
			return
		}
	}

	w.Header().Set("Content-Type", entry.contentType)
	w.Header().Set("Cache-Control", browserCacheControl(s.config.ImgCache.BrowserMaxAge))
	if _, err := w.Write(entry.body); err != nil {
		logger.Error(ctx, "Failed to write img-proxy resp", "err", err)
	}
}

// fetchPoster performs one upstream poster fetch. It is the only seam between
// the imgProxyHandler and TMDB; the cache wraps it so concurrent identical
// requests collapse to a single call.
//
// Only 200 OK with a `Content-Type: image/*` payload that fits within
// maxImgBytes is treated as a successful entry. Anything else is converted
// to errUpstreamNotFound (for upstream 404) or errUpstreamFailed so the
// caller's lrucache will not store it.
func (s *Server) fetchPoster(ctx context.Context, path, size string) (ImgCacheEntry, error) {
	url := tvmeta.GetURLByPosterPathWithSize(path, size)

	//nolint:gosec // url is built from a TMDB poster path; this handler intentionally proxies that fixed host
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logger.Error(ctx, "Failed to create img-proxy request", "err", err)
		return ImgCacheEntry{}, fmt.Errorf("%w: build request: %w", errUpstreamFailed, err)
	}
	//nolint:gosec // url is built from a TMDB poster path; this handler intentionally proxies that fixed host
	resp, err := s.imgClient.Do(req)
	if err != nil {
		logger.Error(ctx, "Failed to perform img-proxy request", "err", err)
		return ImgCacheEntry{}, fmt.Errorf("%w: do: %w", errUpstreamFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return ImgCacheEntry{}, errUpstreamNotFound
	}
	if resp.StatusCode != http.StatusOK {
		logger.Error(ctx, "Img upstream returned non-OK", "status", resp.StatusCode)
		return ImgCacheEntry{}, fmt.Errorf("%w: status=%d", errUpstreamFailed, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		logger.Error(ctx, "Img upstream returned non-image content-type", "content_type", contentType)
		return ImgCacheEntry{}, fmt.Errorf("%w: non-image content-type=%q", errUpstreamFailed, contentType)
	}

	// LimitReader caps the read at maxImgBytes+1 so we can detect overflow:
	// reading exactly maxImgBytes is fine; reading more means the upstream
	// is streaming a body larger than our policy allows.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImgBytes+1))
	if err != nil {
		logger.Error(ctx, "Failed to read img-proxy resp body", "err", err)
		return ImgCacheEntry{}, fmt.Errorf("%w: read body: %w", errUpstreamFailed, err)
	}
	if int64(len(body)) > maxImgBytes {
		logger.Error(ctx, "Img upstream body exceeded cap", "size", len(body), "cap", maxImgBytes)
		return ImgCacheEntry{}, fmt.Errorf("%w: oversized body=%d", errUpstreamFailed, len(body))
	}
	if len(body) == 0 {
		return ImgCacheEntry{}, fmt.Errorf("%w: empty body", errUpstreamFailed)
	}

	return ImgCacheEntry{body: body, contentType: contentType}, nil
}

// browserCacheControl renders the Cache-Control header value sent to the
// browser. Falls back to the safe default of `no-store` when the operator
// has set BrowserMaxAge <= 0 — surfacing posters as uncacheable is harmless;
// silently emitting `max-age=0` would invite CDN edge caches to misbehave.
func browserCacheControl(maxAge time.Duration) string {
	if maxAge <= 0 {
		return "no-store"
	}
	return fmt.Sprintf("public, max-age=%d, immutable", int64(maxAge.Seconds()))
}
