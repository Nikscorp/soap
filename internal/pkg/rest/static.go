package rest

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	staticPath          = "/static"
	staticCompressLevel = 5
)

// AddFileServer mounts the SPA bundle at "/". It also gzips text-y assets
// (HTML/CSS/JS/JSON/SVG) on the fly using chi's stdlib-backed compress
// middleware, and emits long-lived Cache-Control headers for fingerprinted
// assets while keeping index.html / service worker uncached.
func AddFileServer(r *chi.Mux) {
	compress := middleware.Compress(staticCompressLevel,
		"text/html",
		"text/css",
		"text/plain",
		"text/javascript",
		"application/javascript",
		"application/json",
		"application/manifest+json",
		"image/svg+xml",
	)

	fileSystem := customFileSystem{http.Dir(staticPath)}
	fileServer := http.FileServer(fileSystem)

	r.Group(func(r chi.Router) {
		r.Use(compress)
		r.Use(staticCacheControl)
		r.Handle("/*", fileServer)
	})
}

// staticCacheControl sets long-lived immutable caching on Vite's
// content-hashed assets (under /assets/) and on Workbox's hashed runtime
// (workbox-*.js), while keeping the SPA shell, manifest, and main service
// worker uncached so deploys propagate immediately.
func staticCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasPrefix(path, "/assets/"),
			strings.HasPrefix(path, "/workbox-"):
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case path == "/", strings.HasSuffix(path, "/index.html"),
			path == "/sw.js", path == "/registerSW.js", path == "/manifest.webmanifest":
			w.Header().Set("Cache-Control", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}

type customFileSystem struct {
	fs http.FileSystem
}

func (cfs customFileSystem) Open(path string) (http.File, error) {
	f, err := cfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if s.IsDir() {
		index := strings.TrimSuffix(path, "/") + "/index.html"
		if _, err := cfs.fs.Open(index); err != nil {
			return nil, err
		}
	}

	return f, nil
}
