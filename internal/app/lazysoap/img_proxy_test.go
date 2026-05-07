package lazysoap

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nikscorp/soap/internal/app/lazysoap/mocks"
	"github.com/Nikscorp/soap/internal/pkg/lrucache"
	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/stretchr/testify/require"
)

// RoundTripFunc adapts a function to http.RoundTripper for the test
// transport. Returning a nil *http.Response simulates a transport-level
// failure.
type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := f(req)
	if resp == nil {
		return nil, errors.New("transport error: nil response")
	}
	return resp, nil
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: RoundTripFunc(fn),
	}
}

// imageHeader returns a header map with Content-Type: image/jpeg set so the
// /img cache treats the response as a valid poster. Used by every success-path
// fixture; error fixtures pass the bare http.Header to test the rejection path.
func imageHeader() http.Header {
	h := make(http.Header)
	h.Set("Content-Type", "image/jpeg")
	return h
}

func NewServerWithImgClient(t *testing.T, transport RoundTripFunc) *imgProxyFixture {
	tvMetaClientMock := mocks.NewTvMetaClientMock(t)
	cache := lrucache.New[string, ImgCacheEntry]("img", 64, time.Hour, nil)
	srv := &Server{
		config: Config{
			ImgCache: ImgCacheConfig{
				BrowserMaxAge: 24 * time.Hour,
			},
		},
		tvMeta:         tvMetaClientMock,
		metrics:        rest.NewMetrics(),
		featuredPool: newFeaturedPoolCache(),
		imgClient:      NewTestClient(transport),
		imgCache:       cache,
	}
	ts := httptest.NewServer(srv.newRouter())

	return &imgProxyFixture{
		server:           ts,
		tvMetaClientMock: tvMetaClientMock,
		cache:            cache,
	}
}

type imgProxyFixture struct {
	server           *httptest.Server
	tvMetaClientMock *mocks.TvMetaClientMock
	cache            *imgCache
}

func defaultImgTransport(t *testing.T) RoundTripFunc {
	return func(req *http.Request) *http.Response {
		switch path := req.URL.String(); path {
		case "https://image.tmdb.org/t/p/w92/i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg binary data")),
				Header:     imageHeader(),
			}
		case "https://image.tmdb.org/t/p/w92/not_found.jpg":
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBufferString("some html data")),
				Header:     make(http.Header),
			}
		case "https://image.tmdb.org/t/p/w92/internal_error.jpg":
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString("some html data")),
				Header:     make(http.Header),
			}
		case "https://image.tmdb.org/t/p/w92/nil_body.jpg":
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       nil,
				Header:     make(http.Header),
			}
		case "https://image.tmdb.org/t/p/w92/ok_nil_body.jpg":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       nil,
				Header:     imageHeader(),
			}
		case "https://image.tmdb.org/t/p/w92/transport_error.jpg":
			return nil
		case "https://image.tmdb.org/t/p/w342/i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("w342 binary data")),
				Header:     imageHeader(),
			}
		default:
			require.FailNow(t, "invalid path %s", path)
			return nil
		}
	}
}

func TestCommon(t *testing.T) {
	srv := NewServerWithImgClient(t, defaultImgTransport(t))
	defer srv.server.Close()

	tests := []struct {
		name           string
		reqURL         string
		wantStatusCode int
		wantBody       string
	}{
		{
			name:           "positive case",
			reqURL:         "/img/i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg",
			wantStatusCode: http.StatusOK,
			wantBody:       "i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg binary data",
		},
		{
			name:           "not found",
			reqURL:         "/img/not_found.jpg",
			wantStatusCode: http.StatusNotFound,
			wantBody:       "",
		},
		{
			name:           "internal error",
			reqURL:         "/img/internal_error.jpg",
			wantStatusCode: http.StatusBadGateway,
			wantBody:       "",
		},
		{
			name:           "transport error",
			reqURL:         "/img/transport_error.jpg",
			wantStatusCode: http.StatusBadGateway,
			wantBody:       "",
		},
		{
			name:           "nil body (non-OK status)",
			reqURL:         "/img/nil_body.jpg",
			wantStatusCode: http.StatusBadGateway,
			wantBody:       "",
		},
		{
			name:           "nil body (200 OK with image content-type)",
			reqURL:         "/img/ok_nil_body.jpg",
			wantStatusCode: http.StatusBadGateway,
			wantBody:       "",
		},
		{
			name:           "size query selects allowed variant",
			reqURL:         "/img/i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg?size=w342",
			wantStatusCode: http.StatusOK,
			wantBody:       "w342 binary data",
		},
		{
			name:           "disallowed size falls back to default",
			reqURL:         "/img/i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg?size=original",
			wantStatusCode: http.StatusOK,
			wantBody:       "i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg binary data",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, err := http.Get(srv.server.URL + test.reqURL)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			require.Equal(t, test.wantStatusCode, resp.StatusCode)
			require.Equal(t, test.wantBody, string(body))
		})
	}
}

// TestImgProxyCachesAcrossRequests pins down the core observable promise of
// the cache: a second request for the same (path, size) must not produce a
// second upstream call. This is the headline behavior the prewarmer counts
// on — if it regresses, /featured posters pay TMDB latency on every request.
func TestImgProxyCachesAcrossRequests(t *testing.T) {
	var calls atomic.Int32
	transport := func(_ *http.Request) *http.Response {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("payload")),
			Header:     imageHeader(),
		}
	}
	srv := NewServerWithImgClient(t, transport)
	defer srv.server.Close()

	for i := 0; i < 5; i++ {
		resp, err := http.Get(srv.server.URL + "/img/poster.jpg?size=w185")
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "payload", string(body))
	}
	require.EqualValues(t, 1, calls.Load(), "5 sequential requests must collapse to 1 upstream call")
}

// TestImgProxySingleflightCollapse exercises the lrucache singleflight path
// the plan calls out: N concurrent goroutines hitting the same (path, size)
// while the upstream blocks must collapse to a single fetch. Without
// singleflight the prewarmer + organic traffic could fan out N parallel
// TMDB calls under a cold cache.
func TestImgProxySingleflightCollapse(t *testing.T) {
	var calls atomic.Int32
	gate := make(chan struct{})
	transport := func(_ *http.Request) *http.Response {
		calls.Add(1)
		<-gate
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("payload")),
			Header:     imageHeader(),
		}
	}
	srv := NewServerWithImgClient(t, transport)
	defer srv.server.Close()

	const N = 16
	var wg sync.WaitGroup
	wg.Add(N)
	started := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			started <- struct{}{}
			resp, err := http.Get(srv.server.URL + "/img/sf_poster.jpg?size=w342")
			require.NoError(t, err)
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}()
	}
	for i := 0; i < N; i++ {
		<-started
	}
	// Best-effort: give all goroutines a moment to enter singleflight before
	// the upstream returns. This is racy by nature but the LRU+singleflight
	// design tolerates spurious extra calls — the assertion below verifies
	// the actual goal (call count is bounded, not blown up to N).
	time.Sleep(20 * time.Millisecond)
	close(gate)
	wg.Wait()

	require.LessOrEqual(t, calls.Load(), int32(2), "concurrent identical fetches must collapse via singleflight")
}

// TestImgProxyOversizedBodyNotCached verifies the maxImgBytes guard: if the
// upstream streams something past the cap, the request gets a 502 *and* the
// next request is allowed to refetch (no entry was poisoned-in for it).
func TestImgProxyOversizedBodyNotCached(t *testing.T) {
	var calls atomic.Int32
	huge := strings.Repeat("X", int(maxImgBytes)+128)
	transport := func(_ *http.Request) *http.Response {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(huge)),
			Header:     imageHeader(),
		}
	}
	srv := NewServerWithImgClient(t, transport)
	defer srv.server.Close()

	for i := 0; i < 2; i++ {
		resp, err := http.Get(srv.server.URL + "/img/big.jpg?size=w500")
		require.NoError(t, err)
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusBadGateway, resp.StatusCode, "oversized body must surface as 502")
	}
	require.EqualValues(t, 2, calls.Load(), "oversized response must not be cached")
}

// TestImgProxyNonImageContentTypeNotCached covers the "200 OK but garbage"
// upstream: anything outside the image/* family must be rejected and not
// cached, so a transient TMDB outage that returns an HTML error page can't
// freeze that page into the cache.
func TestImgProxyNonImageContentTypeNotCached(t *testing.T) {
	var calls atomic.Int32
	h := make(http.Header)
	h.Set("Content-Type", "text/html; charset=utf-8")
	transport := func(_ *http.Request) *http.Response {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("<html>oops</html>")),
			Header:     h,
		}
	}
	srv := NewServerWithImgClient(t, transport)
	defer srv.server.Close()

	for i := 0; i < 2; i++ {
		resp, err := http.Get(srv.server.URL + "/img/html.jpg?size=w185")
		require.NoError(t, err)
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusBadGateway, resp.StatusCode)
	}
	require.EqualValues(t, 2, calls.Load(), "non-image responses must not be cached")
}

// TestImgProxyEmitsCacheControlHeader pins the Cache-Control header shape on
// both miss-then-cache and hit paths. Browser caching is the second half of
// the speedup story (server cache → wire cache); a missing header would
// silently disable it.
func TestImgProxyEmitsCacheControlHeader(t *testing.T) {
	transport := func(_ *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("payload")),
			Header:     imageHeader(),
		}
	}
	srv := NewServerWithImgClient(t, transport)
	defer srv.server.Close()

	// First request: miss → fetches, then writes Cache-Control.
	resp, err := http.Get(srv.server.URL + "/img/cc.jpg?size=w342")
	require.NoError(t, err)
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "public, max-age=86400, immutable", resp.Header.Get("Cache-Control"))
	require.Equal(t, "image/jpeg", resp.Header.Get("Content-Type"))

	// Second request: hit → same headers replayed from the cached entry.
	resp2, err := http.Get(srv.server.URL + "/img/cc.jpg?size=w342")
	require.NoError(t, err)
	_, _ = io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	require.Equal(t, "public, max-age=86400, immutable", resp2.Header.Get("Cache-Control"))
	require.Equal(t, "image/jpeg", resp2.Header.Get("Content-Type"))
}

// TestImgProxyDisallowedSizeSharesCacheSlot verifies the normalization
// contract documented on imgCacheKey: ?size=garbage and ?size=w92 must
// resolve to the same cache slot so a malicious or buggy client can't churn
// the LRU with one-off entries that all proxy the same upstream URL.
func TestImgProxyDisallowedSizeSharesCacheSlot(t *testing.T) {
	var calls atomic.Int32
	transport := func(_ *http.Request) *http.Response {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("payload")),
			Header:     imageHeader(),
		}
	}
	srv := NewServerWithImgClient(t, transport)
	defer srv.server.Close()

	for _, sizeParam := range []string{"", "garbage", "w92"} {
		url := srv.server.URL + "/img/share.jpg"
		if sizeParam != "" {
			url += "?size=" + sizeParam
		}
		resp, err := http.Get(url)
		require.NoError(t, err)
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	require.EqualValues(t, 1, calls.Load(), "unnormalized + normalized + default must share one entry")
}

// TestImgProxyEmptyBodyReturns502 verifies that a 200 OK upstream response with
// a zero-length body is rejected as a 502 and not cached. An empty image body
// is not a valid poster; caching it would serve a blank body forever until TTL.
func TestImgProxyEmptyBodyReturns502(t *testing.T) {
	var calls atomic.Int32
	transport := func(_ *http.Request) *http.Response {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     imageHeader(),
		}
	}
	srv := NewServerWithImgClient(t, transport)
	defer srv.server.Close()

	for i := 0; i < 2; i++ {
		resp, err := http.Get(srv.server.URL + "/img/empty.jpg?size=w185")
		require.NoError(t, err)
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusBadGateway, resp.StatusCode, "empty body must surface as 502")
	}
	require.EqualValues(t, 2, calls.Load(), "empty body must not be cached")
}

// TestImgProxyBrowserMaxAgeZeroEmitsNoStore verifies that BrowserMaxAge=0
// produces Cache-Control: no-store rather than max-age=0 or a missing header.
func TestImgProxyBrowserMaxAgeZeroEmitsNoStore(t *testing.T) {
	transport := func(_ *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("payload")),
			Header:     imageHeader(),
		}
	}
	tvMetaClientMock := mocks.NewTvMetaClientMock(t)
	cache := lrucache.New[string, ImgCacheEntry]("img", 64, time.Hour, nil)
	srv := &Server{
		config:       Config{ImgCache: ImgCacheConfig{BrowserMaxAge: 0}},
		tvMeta:       tvMetaClientMock,
		metrics:      rest.NewMetrics(),
		featuredPool: newFeaturedPoolCache(),
		imgClient:    NewTestClient(RoundTripFunc(transport)),
		imgCache:     cache,
	}
	ts := httptest.NewServer(srv.newRouter())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/img/noca.jpg?size=w342")
	require.NoError(t, err)
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
}
