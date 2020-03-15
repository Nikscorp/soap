package main

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

func addFileServer(r *mux.Router) {
	fileSystem := customFileSystem{http.Dir(staticPath)}
	fileServer := http.FileServer(fileSystem)
	handler := http.StripPrefix(staticURI, fileServer)
	r.PathPrefix(staticURI).Handler(handler)
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
