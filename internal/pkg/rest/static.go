package rest

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

const (
	staticPath = "/static"
)

func AddFileServer(r *chi.Mux) {
	fileSystem := customFileSystem{http.Dir(staticPath)}
	fileServer := http.FileServer(fileSystem)

	swagger := customFileSystem{http.Dir("/swagger")}
	swaggerServer := http.FileServer(swagger)
	r.Handle("/swagger/*", http.StripPrefix("/swagger/", swaggerServer))
	r.Handle("/*", fileServer)
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
