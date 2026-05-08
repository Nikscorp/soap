package lazysoap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nikscorp/soap/internal/app/lazysoap/mocks"
	"github.com/Nikscorp/soap/internal/pkg/lrucache"
	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/stretchr/testify/require"
)

type featuredFixture struct {
	server *httptest.Server
	srv    *Server
	mock   *mocks.TvMetaClientMock
}

func (f *featuredFixture) close() { f.server.Close() }

// warmPool populates the in-memory pool the same way the background refresher
// would. Tests call this after wiring up PopularTVShowsMock and
// TVShowDetailsMock so the handler can hit the cached pool without
// round-tripping TMDB.
func (f *featuredFixture) warmPool(t *testing.T) {
	t.Helper()
	f.srv.refreshFeaturedPool(context.Background())
}

func newFeaturedFixture(t *testing.T, cfg Config) *featuredFixture {
	tvMetaClientMock := mocks.NewTvMetaClientMock(t)
	srv := New(cfg, tvMetaClientMock, nil, "")
	ts := httptest.NewServer(srv.newRouter())
	return &featuredFixture{server: ts, srv: srv, mock: tvMetaClientMock}
}

func TestFeaturedHandlerUnionDedupAndCount(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedCount:    3,
		FeaturedExtraIDs: []int{100, 200, 1}, // 1 overlaps with popular pool
	})
	defer f.close()

	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		return []*tvmeta.TVShow{
			{ID: 1, Name: "Popular One", PosterLink: "/img/p1.jpg", FirstAirDate: "2020"},
			{ID: 2, Name: "Popular Two", PosterLink: "/img/p2.jpg", FirstAirDate: "2021"},
		}, nil
	})

	f.mock.TVShowDetailsMock.Set(func(_ context.Context, id int, _ string) (*tvmeta.TvShowDetails, error) {
		switch id {
		case 100:
			return &tvmeta.TvShowDetails{ID: 100, Title: "Extra A", PosterLink: "/img/a.jpg", FirstAirDate: "2010"}, nil
		case 200:
			return &tvmeta.TvShowDetails{ID: 200, Title: "Extra B", PosterLink: "/img/b.jpg", FirstAirDate: "2011"}, nil
		case 1:
			return &tvmeta.TvShowDetails{ID: 1, Title: "Curated One", PosterLink: "/img/c1.jpg", FirstAirDate: "2020"}, nil
		}
		t.Fatalf("unexpected id %d", id)
		return nil, nil
	})

	f.warmPool(t)
	popularCallsAfterWarm := len(f.mock.PopularTVShowsMock.Calls())
	detailsCallsAfterWarm := len(f.mock.TVShowDetailsMock.Calls())
	require.Equal(t, 1, popularCallsAfterWarm, "warm should fetch popular exactly once")
	require.Equal(t, 3, detailsCallsAfterWarm, "warm should fetch each extra ID exactly once")

	resp, err := http.Get(f.server.URL + "/featured?language=en")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "no-store", resp.Header.Get("Cache-Control"))

	var got featuredResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, "en", got.Language)
	require.Len(t, got.Series, 3)

	ids := make([]int, 0, len(got.Series))
	for _, s := range got.Series {
		ids = append(ids, s.ID)
	}
	sort.Ints(ids)
	// All four candidates union to {1,2,100,200}; we expect 3 unique picks from that set.
	require.Subset(t, []int{1, 2, 100, 200}, ids)
	require.Len(t, uniqueInts(ids), 3)
	// The handler MUST NOT call TMDB: pool is served from cache.
	require.Equal(t, popularCallsAfterWarm, len(f.mock.PopularTVShowsMock.Calls()))
	require.Equal(t, detailsCallsAfterWarm, len(f.mock.TVShowDetailsMock.Calls()))

	// Popular wins on ID collision: ID=1 should carry the popular title.
	for _, s := range got.Series {
		if s.ID == 1 {
			require.Equal(t, "Popular One", s.Title)
		}
	}
}

func TestFeaturedHandlerPopularErrorFallsBackToExtras(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedCount:    2,
		FeaturedExtraIDs: []int{100, 200},
	})
	defer f.close()

	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		return nil, errors.New("tmdb down")
	})
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, id int, _ string) (*tvmeta.TvShowDetails, error) {
		return &tvmeta.TvShowDetails{ID: id, Title: "Extra", PosterLink: "/img/x.jpg", FirstAirDate: "2000"}, nil
	})

	// Bootstrap path: no prior pool, popular fails, extras succeed → publish
	// extras-only pool so /featured isn't stuck at 503 forever.
	f.warmPool(t)

	resp, err := http.Get(f.server.URL + "/featured?language=en")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got featuredResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Len(t, got.Series, 2)
}

func TestFeaturedHandlerSkipsExtraDetailsErrors(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedCount:    2,
		FeaturedExtraIDs: []int{100, 200, 300},
	})
	defer f.close()

	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		return nil, nil
	})
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, id int, _ string) (*tvmeta.TvShowDetails, error) {
		if id == 200 {
			return nil, errors.New("not found")
		}
		return &tvmeta.TvShowDetails{ID: id, Title: "Extra", PosterLink: "/img/x.jpg", FirstAirDate: "2000"}, nil
	})

	f.warmPool(t)

	resp, err := http.Get(f.server.URL + "/featured?language=en")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got featuredResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	// Two healthy extras (100, 300) survive; one bad (200) was dropped.
	require.Len(t, got.Series, 2)
	for _, s := range got.Series {
		require.Contains(t, []int{100, 300}, s.ID)
	}
}

func TestFeaturedHandlerPoolSmallerThanCountReturns503(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedCount:    3,
		FeaturedExtraIDs: []int{100, 200},
	})
	defer f.close()

	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		return nil, nil
	})
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, id int, _ string) (*tvmeta.TvShowDetails, error) {
		return &tvmeta.TvShowDetails{ID: id, Title: "Extra", PosterLink: "/img/x.jpg", FirstAirDate: "2000"}, nil
	})

	f.warmPool(t)

	resp, err := http.Get(f.server.URL + "/featured?language=en")
	require.NoError(t, err)
	defer resp.Body.Close()
	// Pool of 2 < count of 3 → 503 instead of returning a short list.
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestFeaturedHandlerFiltersLowVoteCountFromPopular(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedCount:        3,
		FeaturedMinVoteCount: 100,
		FeaturedExtraIDs:     []int{},
	})
	defer f.close()

	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		return []*tvmeta.TVShow{
			{ID: 1, Name: "Spam", VoteCount: 0, PosterLink: "/img/1.jpg", FirstAirDate: "2020"},
			{ID: 2, Name: "Real Hit A", VoteCount: 5000, PosterLink: "/img/2.jpg", FirstAirDate: "2020"},
			{ID: 3, Name: "Borderline", VoteCount: 99, PosterLink: "/img/3.jpg", FirstAirDate: "2020"},
			{ID: 4, Name: "Real Hit B", VoteCount: 200, PosterLink: "/img/4.jpg", FirstAirDate: "2020"},
			{ID: 5, Name: "Real Hit C", VoteCount: 100, PosterLink: "/img/5.jpg", FirstAirDate: "2020"},
		}, nil
	})

	f.warmPool(t)

	resp, err := http.Get(f.server.URL + "/featured?language=en")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got featuredResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Len(t, got.Series, 3)
	for _, s := range got.Series {
		// Only entries with VoteCount >= 100 should be eligible (IDs 2, 4, 5).
		require.Contains(t, []int{2, 4, 5}, s.ID)
	}
}

func TestFeaturedHandlerExtrasBypassVoteCountFilter(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedCount:        2,
		FeaturedMinVoteCount: 1000,
		FeaturedExtraIDs:     []int{100, 200},
	})
	defer f.close()

	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		// Everything below the (very high) threshold; should be fully filtered.
		return []*tvmeta.TVShow{
			{ID: 1, Name: "P1", VoteCount: 5, PosterLink: "/img/1.jpg", FirstAirDate: "2020"},
		}, nil
	})
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, id int, _ string) (*tvmeta.TvShowDetails, error) {
		return &tvmeta.TvShowDetails{ID: id, Title: "Curated", PosterLink: "/img/x.jpg", FirstAirDate: "2000"}, nil
	})

	f.warmPool(t)

	resp, err := http.Get(f.server.URL + "/featured?language=en")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got featuredResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Len(t, got.Series, 2)
	for _, s := range got.Series {
		require.Contains(t, []int{100, 200}, s.ID)
	}
}

func TestFeaturedHandlerEmptyPoolReturns503(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedCount:    3,
		FeaturedExtraIDs: []int{},
	})
	defer f.close()

	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		return nil, errors.New("tmdb down")
	})

	// Bootstrap path with total failure → no pool published, 503.
	f.warmPool(t)

	resp, err := http.Get(f.server.URL + "/featured?language=en")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "", string(body))
}

func TestFeaturedHandlerNeverHitsTMDBOnRequestPath(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedCount:    1,
		FeaturedExtraIDs: []int{100, 200, 300},
	})
	defer f.close()

	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		return nil, nil
	})
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, id int, _ string) (*tvmeta.TvShowDetails, error) {
		return &tvmeta.TvShowDetails{ID: id, Title: "Extra", PosterLink: "/img/x.jpg", FirstAirDate: "2000"}, nil
	})

	f.warmPool(t)
	beforePopular := len(f.mock.PopularTVShowsMock.Calls())
	beforeDetails := len(f.mock.TVShowDetailsMock.Calls())

	for i := 0; i < 25; i++ {
		resp, err := http.Get(f.server.URL + "/featured?language=en")
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// 25 requests must not have produced a single new TMDB call: the pool is
	// fully served from the cached unified slice.
	require.Equal(t, beforeDetails, len(f.mock.TVShowDetailsMock.Calls()),
		"request path must serve pool from cache, never call TVShowDetails")
	require.Equal(t, beforePopular, len(f.mock.PopularTVShowsMock.Calls()),
		"request path must serve pool from cache, never call PopularTVShows")
}

func TestRefreshFeaturedPoolKeepsPriorOnTotalFailure(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedCount:    2,
		FeaturedExtraIDs: []int{100, 200},
	})
	defer f.close()

	// First refresh: both succeed → pool populated.
	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		return []*tvmeta.TVShow{
			{ID: 1, Name: "P1", VoteCount: 100, PosterLink: "/img/p1.jpg", FirstAirDate: "2020"},
		}, nil
	})
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, id int, _ string) (*tvmeta.TvShowDetails, error) {
		return &tvmeta.TvShowDetails{ID: id, Title: "Good", PosterLink: "/img/g.jpg", FirstAirDate: "2000"}, nil
	})
	f.srv.refreshFeaturedPool(context.Background())
	priorLen := len(f.srv.featuredPool.view())
	require.Equal(t, 3, priorLen)

	// Second refresh: every TMDB call errors → previous pool kept intact.
	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		return nil, errors.New("tmdb down")
	})
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, _ int, _ string) (*tvmeta.TvShowDetails, error) {
		return nil, errors.New("tmdb down")
	})
	f.srv.refreshFeaturedPool(context.Background())
	require.Len(t, f.srv.featuredPool.view(), priorLen,
		"all-failure refresh must not wipe a healthy pool")
}

func TestRefreshFeaturedPoolPopularFailurePreservesPriorPool(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedCount:    2,
		FeaturedExtraIDs: []int{100, 200},
	})
	defer f.close()

	// First refresh: both succeed → pool populated with popular + extras.
	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		return []*tvmeta.TVShow{
			{ID: 1, Name: "P1", VoteCount: 100, PosterLink: "/img/p1.jpg", FirstAirDate: "2020"},
		}, nil
	})
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, id int, _ string) (*tvmeta.TvShowDetails, error) {
		return &tvmeta.TvShowDetails{ID: id, Title: "Good", PosterLink: "/img/g.jpg", FirstAirDate: "2000"}, nil
	})
	f.srv.refreshFeaturedPool(context.Background())
	priorPool := append([]featuredItem(nil), f.srv.featuredPool.view()...)
	require.Len(t, priorPool, 3)

	// Second refresh: popular fails, extras succeed → prior pool preserved.
	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		return nil, errors.New("popular down")
	})
	f.srv.refreshFeaturedPool(context.Background())

	require.Equal(t, priorPool, f.srv.featuredPool.view(),
		"popular-fetch failure must preserve previously-warmed pool")
}

func TestRefreshFeaturedPoolExtrasFailurePreservesPriorPool(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedCount:    2,
		FeaturedExtraIDs: []int{100, 200},
	})
	defer f.close()

	// First refresh: both succeed.
	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		return []*tvmeta.TVShow{
			{ID: 1, Name: "P1", VoteCount: 100, PosterLink: "/img/p1.jpg", FirstAirDate: "2020"},
		}, nil
	})
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, id int, _ string) (*tvmeta.TvShowDetails, error) {
		return &tvmeta.TvShowDetails{ID: id, Title: "Good", PosterLink: "/img/g.jpg", FirstAirDate: "2000"}, nil
	})
	f.srv.refreshFeaturedPool(context.Background())
	priorPool := append([]featuredItem(nil), f.srv.featuredPool.view()...)
	require.Len(t, priorPool, 3)

	// Second refresh: every extra ID fails → prior pool preserved.
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, _ int, _ string) (*tvmeta.TvShowDetails, error) {
		return nil, errors.New("extras down")
	})
	f.srv.refreshFeaturedPool(context.Background())

	require.Equal(t, priorPool, f.srv.featuredPool.view(),
		"extras-fetch failure must preserve previously-warmed pool")
}

func TestRefreshFeaturedPoolUnifiesPopularAndExtras(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedExtraIDs: []int{100, 200, 1}, // 1 collides with popular
	})
	defer f.close()

	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		return []*tvmeta.TVShow{
			{ID: 1, Name: "Popular One", PosterLink: "/img/p1.jpg", FirstAirDate: "2020"},
			{ID: 2, Name: "Popular Two", PosterLink: "/img/p2.jpg", FirstAirDate: "2021"},
		}, nil
	})
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, id int, _ string) (*tvmeta.TvShowDetails, error) {
		return &tvmeta.TvShowDetails{ID: id, Title: "Curated", PosterLink: "/img/x.jpg", FirstAirDate: "2000"}, nil
	})

	f.srv.refreshFeaturedPool(context.Background())
	view := f.srv.featuredPool.view()

	ids := make([]int, 0, len(view))
	for _, item := range view {
		ids = append(ids, item.ID)
	}
	sort.Ints(ids)
	require.Equal(t, []int{1, 2, 100, 200}, ids, "unified pool with dedup")

	// Popular wins on ID collision: ID=1 should carry the popular title.
	for _, item := range view {
		if item.ID == 1 {
			require.Equal(t, "Popular One", item.Title)
		}
	}
}

func TestRunFeaturedPoolRefreshTicks(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedExtraIDs:              []int{100},
		FeaturedExtrasRefreshInterval: 30 * time.Millisecond,
	})
	defer f.close()

	var popularCalls atomic.Int32
	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		popularCalls.Add(1)
		return nil, nil
	})
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, id int, _ string) (*tvmeta.TvShowDetails, error) {
		return &tvmeta.TvShowDetails{ID: id, Title: "T", PosterLink: "/img/t.jpg", FirstAirDate: "2000"}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		f.srv.runFeaturedPoolRefresh(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool { return popularCalls.Load() >= 3 }, 1*time.Second, 10*time.Millisecond,
		"expected at least 3 refreshes (initial + 2 ticks)")

	cancel()
	<-done
}

func TestRunFeaturedPoolRefreshNoExtrasStillFetchesPopular(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedExtraIDs:              []int{},
		FeaturedExtrasRefreshInterval: 0, // single initial refresh, no ticker
	})
	defer f.close()

	var popularCalls atomic.Int32
	f.mock.PopularTVShowsMock.Set(func(_ context.Context, _ string) ([]*tvmeta.TVShow, error) {
		popularCalls.Add(1)
		return []*tvmeta.TVShow{
			{ID: 1, Name: "P1", VoteCount: 100, PosterLink: "/img/p1.jpg", FirstAirDate: "2020"},
		}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		f.srv.runFeaturedPoolRefresh(ctx)
		close(done)
	}()
	<-done

	// Initial refresh fired exactly once, then exited because interval is 0.
	require.Equal(t, int32(1), popularCalls.Load())
	// Pool was published.
	require.Len(t, f.srv.featuredPool.view(), 1)
}

// newPrewarmServer builds a Server wired with the pinned featuredImgs tier +
// a real imgCache + custom transport so prewarmFeaturedImages can be exercised
// end-to-end (fetchPoster code path, errgroup bound, atomic swap of the
// pinned map). The router is not used — tests call prewarmFeaturedImages
// directly.
//
// The imgCache is included so the fixture matches production wiring, but
// prewarm no longer writes there — assertions about prewarm output should
// target srv.featuredImgs.
func newPrewarmServer(t *testing.T, transport RoundTripFunc) (*Server, *imgCache) {
	t.Helper()
	cache := lrucache.New[string, ImgCacheEntry]("img", 256, time.Hour, nil)
	srv := &Server{
		config: Config{
			ImgCache: ImgCacheConfig{BrowserMaxAge: time.Hour},
		},
		tvMeta:       mocks.NewTvMetaClientMock(t),
		metrics:      rest.NewMetrics(),
		featuredPool: newFeaturedPoolCache(),
		featuredImgs: newFeaturedImgCache(),
		imgClient:    NewTestClient(transport),
		imgCache:     cache,
	}
	return srv, cache
}

func TestPrewarmFeaturedImagesWarmsAllSizes(t *testing.T) {
	var calls atomic.Int32
	transport := func(_ *http.Request) *http.Response {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("payload")),
			Header:     imageHeader(),
		}
	}
	srv, cache := newPrewarmServer(t, transport)

	pool := []featuredItem{
		{ID: 1, Poster: "/img/p1.jpg"},
		{ID: 2, Poster: "/img/p2.jpg"},
		{ID: 3, Poster: "/img/p3.jpg"},
	}

	srv.prewarmFeaturedImages(context.Background(), pool)

	want := len(pool) * len(prewarmSizes)
	require.Equal(t, want, srv.featuredImgs.len(),
		"prewarm should populate one pinned entry per (item, size) on the happy path")
	require.Equal(t, 0, cache.Len(),
		"prewarm must NOT touch the LRU — featured images live exclusively in featuredImgs")
	require.EqualValues(t, want, calls.Load(),
		"each unique (path, size) must round-trip TMDB exactly once")
}

func TestPrewarmFeaturedImagesContinuesOnFailingPoster(t *testing.T) {
	var calls atomic.Int32
	transport := func(req *http.Request) *http.Response {
		calls.Add(1)
		if strings.Contains(req.URL.Path, "bad.jpg") {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBufferString("")),
				Header:     make(http.Header),
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("payload")),
			Header:     imageHeader(),
		}
	}
	srv, cache := newPrewarmServer(t, transport)

	pool := []featuredItem{
		{ID: 1, Poster: "/img/good1.jpg"},
		{ID: 2, Poster: "/img/bad.jpg"},
		{ID: 3, Poster: "/img/good2.jpg"},
	}

	srv.prewarmFeaturedImages(context.Background(), pool)

	// Two healthy posters × len(prewarmSizes) end up pinned. The bad one's
	// per-size 404s are NOT pinned (failed fetches are logged and dropped).
	wantPinned := 2 * len(prewarmSizes)
	require.Equal(t, wantPinned, srv.featuredImgs.len(),
		"failures on one poster must not block warming the rest")
	require.Equal(t, 0, cache.Len(),
		"prewarm must NOT touch the LRU — failures or successes")
	require.EqualValues(t, len(pool)*len(prewarmSizes), calls.Load(),
		"every (path, size) pair is attempted, even the failing ones")
}

func TestPrewarmFeaturedImagesSkipsItemsWithoutPosterPrefix(t *testing.T) {
	var calls atomic.Int32
	transport := func(_ *http.Request) *http.Response {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("payload")),
			Header:     imageHeader(),
		}
	}
	srv, cache := newPrewarmServer(t, transport)

	pool := []featuredItem{
		{ID: 1, Poster: ""},                 // no poster at all
		{ID: 2, Poster: "/img/"},            // empty trailing path
		{ID: 3, Poster: "https://x/y.jpg"},  // unexpected shape
		{ID: 4, Poster: "/img/healthy.jpg"}, // only this one is warmable
	}

	srv.prewarmFeaturedImages(context.Background(), pool)

	require.Equal(t, len(prewarmSizes), srv.featuredImgs.len(),
		"only the well-formed /img/{path} entry should warm")
	require.Equal(t, 0, cache.Len(),
		"prewarm must NOT touch the LRU regardless of input shape")
	require.EqualValues(t, len(prewarmSizes), calls.Load(),
		"malformed posters must not produce upstream calls")
}

func TestPrewarmFeaturedImagesContextCancellationReturnsPromptly(t *testing.T) {
	gate := make(chan struct{})
	var inflight atomic.Int32
	transport := func(_ *http.Request) *http.Response {
		inflight.Add(1)
		<-gate
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("payload")),
			Header:     imageHeader(),
		}
	}
	srv, _ := newPrewarmServer(t, transport)

	// Big enough to saturate prewarmConcurrency several times over so the
	// loop is forced to wait on errgroup slots.
	pool := make([]featuredItem, prewarmConcurrency*4)
	for i := range pool {
		pool[i] = featuredItem{ID: i + 1, Poster: "/img/p" + strings.Repeat("x", i+1) + ".jpg"}
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		srv.prewarmFeaturedImages(ctx, pool)
		close(done)
	}()

	// Wait until the transport actually has goroutines parked so the cancel
	// happens after the bounded loop fills its slots.
	require.Eventually(t, func() bool { return inflight.Load() >= int32(prewarmConcurrency) },
		1*time.Second, 5*time.Millisecond,
		"expected prewarm to saturate its concurrency limit before cancel")

	cancel()
	// Unblock the transport so the in-progress fetches (those that had already
	// entered GetOrFetch before cancellation) can finish cleanly.
	close(gate)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("prewarmFeaturedImages did not return promptly after ctx cancellation")
	}
}

func uniqueInts(ints []int) []int {
	seen := make(map[int]struct{}, len(ints))
	out := make([]int, 0, len(ints))
	for _, v := range ints {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
