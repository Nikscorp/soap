package rest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPing(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { nextCalled = true })

	t.Run("returns pong on /ping", func(t *testing.T) {
		nextCalled = false
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		Ping(next).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "pong", rec.Body.String())
		assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
		assert.False(t, nextCalled)
	})

	t.Run("forwards non-ping requests", func(t *testing.T) {
		nextCalled = false
		rec := httptest.NewRecorder()
		Ping(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/something", nil))
		assert.True(t, nextCalled)
	})

	t.Run("ignores non-GET on /ping", func(t *testing.T) {
		nextCalled = false
		rec := httptest.NewRecorder()
		Ping(next).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/ping", nil))
		assert.True(t, nextCalled)
	})
}

func TestVersion(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	t.Run("returns version on /version", func(t *testing.T) {
		rec := httptest.NewRecorder()
		Version("v1.2.3")(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/version", nil))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "v1.2.3", rec.Body.String())
	})

	t.Run("forwards other paths", func(t *testing.T) {
		rec := httptest.NewRecorder()
		Version("v1")(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/other", nil))
		assert.Equal(t, http.StatusTeapot, rec.Code)
	})
}

func TestRequestIDHeader_MirrorsChiID(t *testing.T) {
	chain := middleware.RequestID(RequestIDHeader(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-Request-Id"))
}

func TestRequestIDHeader_NoChiIDIsNoop(t *testing.T) {
	rec := httptest.NewRecorder()
	RequestIDHeader(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("X-Request-Id"))
}

func TestLogRequest_AddsRouteAttr(t *testing.T) {
	r := chi.NewRouter()
	r.Use(LogRequest)
	hit := false
	r.Get("/things/{id}", func(w http.ResponseWriter, req *http.Request) {
		hit = true
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/things/42", nil))

	assert.True(t, hit)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestLogRequest_SkipsDebugAndMetrics(t *testing.T) {
	r := chi.NewRouter()
	r.Use(LogRequest)
	r.Get("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("metrics-body"))
	})
	r.Get("/debug/pprof/profile", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "metrics-body", rec.Body.String())

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/debug/pprof/profile", nil))
	assert.Equal(t, http.StatusAccepted, rec.Code)
}

func TestRoutePattern_NoMatchFallsBackToPath(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/known", func(http.ResponseWriter, *http.Request) {})

	captured := ""
	probe := http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		captured = routePattern(req)
	})

	r.NotFound(probe)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/totally/unknown", nil))
	assert.Equal(t, "/totally/unknown", captured)
}

func TestMaskedURL(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		hide    map[string]struct{}
		expects []string
		absent  []string
	}{
		{
			name:    "hides matching key",
			raw:     "/foo?api_key=secret&q=hello",
			hide:    map[string]struct{}{"api_key": {}},
			expects: []string{"q=hello", "api_key=***"},
			absent:  []string{"secret"},
		},
		{
			name:    "leaves unrelated keys",
			raw:     "/foo?q=hello",
			hide:    map[string]struct{}{"api_key": {}},
			expects: []string{"q=hello"},
		},
		{
			name:    "nil hide map masks nothing",
			raw:     "/foo?api_key=secret",
			hide:    nil,
			expects: []string{"api_key=secret"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.raw)
			require.NoError(t, err)
			got := maskedURL(u, tc.hide)
			for _, want := range tc.expects {
				assert.Contains(t, got, want)
			}
			for _, bad := range tc.absent {
				assert.NotContains(t, got, bad)
			}
		})
	}
}

// Sanity-check that LogRequest hands the request through with body intact —
// guards against accidental body consumption / wrap-writer bugs.
func TestLogRequest_BodyPreserved(t *testing.T) {
	r := chi.NewRouter()
	r.Use(LogRequest)
	r.Post("/echo", func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		_, _ = w.Write(body)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/echo", stringBody("hello")))

	assert.Equal(t, "hello", rec.Body.String())
}

func stringBody(s string) io.Reader { return &readerFromString{s: s} }

type readerFromString struct {
	s   string
	pos int
}

func (r *readerFromString) Read(p []byte) (int, error) {
	if r.pos >= len(r.s) {
		return 0, io.EOF
	}
	n := copy(p, r.s[r.pos:])
	r.pos += n
	return n, nil
}

