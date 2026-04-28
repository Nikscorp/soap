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

// warmExtras populates the in-memory extras cache the same way the background
// refresher would. Tests call this after wiring up TVShowDetailsMock so the
// handler can hit the cache without round-tripping TMDB.
func (f *featuredFixture) warmExtras(t *testing.T) {
	t.Helper()
	f.srv.refreshFeaturedExtras(context.Background())
}

func newFeaturedFixture(t *testing.T, cfg Config) *featuredFixture {
	tvMetaClientMock := mocks.NewTvMetaClientMock(t)
	srv := New(cfg, tvMetaClientMock, "")
	ts := httptest.NewServer(srv.newRouter())
	return &featuredFixture{server: ts, srv: srv, mock: tvMetaClientMock}
}

func TestFeaturedHandlerUnionDedupAndCount(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedCount:    3,
		FeaturedExtraIDs: []int{100, 200, 1}, // 1 overlaps with popular pool
	})
	defer f.close()

	f.mock.PopularTVShowsMock.Set(func(_ context.Context, language string) ([]*tvmeta.TVShow, error) {
		require.Equal(t, "en", language)
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

	f.warmExtras(t)
	detailsCallsAfterWarm := len(f.mock.TVShowDetailsMock.Calls())
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
	require.Equal(t, 1, len(f.mock.PopularTVShowsMock.Calls()))
	// The handler MUST NOT call TVShowDetails: extras are served from cache.
	require.Equal(t, detailsCallsAfterWarm, len(f.mock.TVShowDetailsMock.Calls()))
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

	f.warmExtras(t)

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

	f.warmExtras(t)

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

	f.warmExtras(t)

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

	f.warmExtras(t)

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

	resp, err := http.Get(f.server.URL + "/featured?language=en")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "", string(body))
}

func TestFeaturedHandlerNeverHitsTMDBForExtrasOnRequestPath(t *testing.T) {
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

	f.warmExtras(t)
	beforeRequests := len(f.mock.TVShowDetailsMock.Calls())

	for i := 0; i < 25; i++ {
		resp, err := http.Get(f.server.URL + "/featured?language=en")
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// The 25 requests must not have produced a single new TVShowDetails call.
	require.Equal(t, beforeRequests, len(f.mock.TVShowDetailsMock.Calls()),
		"request path must serve extras from cache, never call TMDB")
	// PopularTVShows is still called once per request (it changes daily).
	require.Equal(t, 25, len(f.mock.PopularTVShowsMock.Calls()))
}

func TestRefreshFeaturedExtrasKeepsPreviousCacheOnTotalFailure(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedCount:    2,
		FeaturedExtraIDs: []int{100, 200},
	})
	defer f.close()

	// First refresh: both succeed → cache populated with 2 items.
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, id int, _ string) (*tvmeta.TvShowDetails, error) {
		return &tvmeta.TvShowDetails{ID: id, Title: "Good", PosterLink: "/img/g.jpg", FirstAirDate: "2000"}, nil
	})
	f.srv.refreshFeaturedExtras(context.Background())
	require.Len(t, f.srv.featuredExtras.view(), 2)

	// Second refresh: every ID errors → previous cache kept intact.
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, _ int, _ string) (*tvmeta.TvShowDetails, error) {
		return nil, errors.New("tmdb down")
	})
	f.srv.refreshFeaturedExtras(context.Background())
	require.Len(t, f.srv.featuredExtras.view(), 2,
		"all-failure refresh must not wipe a healthy cache")
}

func TestRunFeaturedExtrasRefreshTicks(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedExtraIDs:              []int{100},
		FeaturedExtrasRefreshInterval: 30 * time.Millisecond,
	})
	defer f.close()

	var calls atomic.Int32
	f.mock.TVShowDetailsMock.Set(func(_ context.Context, id int, _ string) (*tvmeta.TvShowDetails, error) {
		calls.Add(1)
		return &tvmeta.TvShowDetails{ID: id, Title: "T", PosterLink: "/img/t.jpg", FirstAirDate: "2000"}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		f.srv.runFeaturedExtrasRefresh(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool { return calls.Load() >= 3 }, 1*time.Second, 10*time.Millisecond,
		"expected at least 3 refreshes (initial + 2 ticks)")

	cancel()
	<-done
}

func TestRunFeaturedExtrasRefreshNoExtrasIsNoOp(t *testing.T) {
	f := newFeaturedFixture(t, Config{
		FeaturedExtraIDs:              []int{},
		FeaturedExtrasRefreshInterval: 5 * time.Millisecond,
	})
	defer f.close()

	// Mock not set; if runFeaturedExtrasRefresh tried to call TVShowDetails,
	// minimock would fail the test on the unexpected call.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		f.srv.runFeaturedExtrasRefresh(ctx)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done
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
