package lazysoap

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nikscorp/soap/internal/app/lazysoap/mocks"
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
