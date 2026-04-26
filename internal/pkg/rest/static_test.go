package rest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticCacheControl(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/assets/main.abc.js", "public, max-age=31536000, immutable"},
		{"/workbox-deadbeef.js", "public, max-age=31536000, immutable"},
		{"/", "no-cache"},
		{"/index.html", "no-cache"},
		{"/sw.js", "no-cache"},
		{"/registerSW.js", "no-cache"},
		{"/manifest.webmanifest", "no-cache"},
		{"/random.txt", ""},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			staticCacheControl(next).ServeHTTP(rec, req)
			assert.Equal(t, tc.want, rec.Header().Get("Cache-Control"))
		})
	}
}

func TestCustomFileSystem_DirectoryWithoutIndex_Returns404(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("hi"), 0o644))

	cfs := customFileSystem{http.Dir(dir)}

	f, err := cfs.Open("/ok.txt")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	_, err = cfs.Open("/sub")
	assert.Error(t, err, "directory without index.html should fail to open")

	_, err = cfs.Open("/no-such-file")
	assert.Error(t, err)
}

func TestCustomFileSystem_DirectoryWithIndex_Succeeds(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "page")
	require.NoError(t, os.Mkdir(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "index.html"), []byte("<html/>"), 0o644))

	cfs := customFileSystem{http.Dir(dir)}
	f, err := cfs.Open("/page")
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func TestAddFileServer_DoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		AddFileServer(chi.NewRouter())
	})
}
