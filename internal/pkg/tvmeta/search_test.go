package tvmeta

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/tvmeta/mocks"
	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// searchCacheCfg returns a CacheConfig with the search cache enabled at a
// generous size and TTL — large enough that no test-grade timing flake can
// expire entries mid-test.
func searchCacheCfg() CacheConfig {
	return CacheConfig{SearchSize: 64, SearchTTL: time.Minute}
}

type ClientM struct {
	client        *Client
	mockedTMDB    *mocks.TmdbClientMock
	mockedRatings *mocks.RatingsProviderMock
}

// NewClientM constructs a Client wired to a fresh tmdbClient mock and a
// RatingsProvider mock that defaults to "not ready" (so existing tests
// observe the legacy TMDB-only path without any extra setup). Tests that
// exercise the IMDb-overlay path can call .Ready/.SeriesRating/.EpisodeRating
// expectations on m.mockedRatings.
//
// The Client is constructed with a zero CacheConfig, which disables every
// per-method cache (pass-through) — keeps existing tests deterministic. Tests
// that exercise caching behavior should use NewClientMCfg.
func NewClientM(t *testing.T) *ClientM {
	return NewClientMCfg(t, CacheConfig{})
}

// NewClientMCfg is NewClientM with an explicit CacheConfig, for tests that
// exercise the per-method response caches. Metrics are disabled (nil
// registerer) so existing tests don't have to manage Prometheus registry
// state. Tests that assert on cache observability should call
// NewClientMCfgRegisterer with a fresh prometheus.NewRegistry.
func NewClientMCfg(t *testing.T, cacheCfg CacheConfig) *ClientM {
	return NewClientMCfgRegisterer(t, cacheCfg, nil)
}

// NewClientMCfgRegisterer is NewClientMCfg with an explicit prometheus
// registerer. Tests that need to assert on tvmeta_cache_*_total counters
// should pass a per-test prometheus.NewRegistry and gather from it for
// isolation from other tests in the suite.
func NewClientMCfgRegisterer(t *testing.T, cacheCfg CacheConfig, registerer prometheus.Registerer) *ClientM {
	tmdbClient := mocks.NewTmdbClientMock(t)
	ratings := mocks.NewRatingsProviderMock(t)
	ratings.ReadyMock.Return(false)
	client := New(tmdbClient, ratings, cacheCfg, registerer)
	return &ClientM{
		client:        client,
		mockedTMDB:    tmdbClient,
		mockedRatings: ratings,
	}
}

func TestSearchTVShows(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
		require.Equal(t, "Lost", query)
		require.Equal(t, 1, len(urlOptions))
		require.Equal(t, enLangTag, urlOptions["language"])

		return &tmdb.SearchTVShows{
			SearchTVShowsResults: &tmdb.SearchTVShowsResults{
				Results: []tmdb.TVShowResult{
					{
						ID:           4815162343,
						Name:         "Lost2",
						VoteMetrics:  tmdb.VoteMetrics{VoteAverage: 9.9},
						PosterPath:   "/lost.jpg",
						FirstAirDate: "2022",
						Overview:     "Greatest tv show ever",
						Popularity:   1,
					},
					{
						ID:           4815162342,
						Name:         "Lost",
						VoteMetrics:  tmdb.VoteMetrics{VoteAverage: 9.9},
						PosterPath:   "/lost.jpg",
						FirstAirDate: "2022",
						Overview:     "Greatest tv show ever",
						Popularity:   1000,
					},
				},
			},
		}, nil
	})

	resp, err := client.client.SearchTVShows(context.Background(), "Lost")
	require.Equal(t, 1, len(client.mockedTMDB.GetSearchTVShowMock.Calls()))
	require.NoError(t, err)

	// sorted by popularity desc
	require.Equal(t, &TVShows{
		Language: enLangTag,
		TVShows: []*TVShow{
			{
				ID:           4815162342,
				Name:         "Lost",
				Rating:       9.9,
				PosterLink:   "/img/lost.jpg",
				FirstAirDate: "2022",
				Description:  "Greatest tv show ever",
				Popularity:   1000,
			},
			{
				ID:           4815162343,
				Name:         "Lost2",
				Rating:       9.9,
				PosterLink:   "/img/lost.jpg",
				FirstAirDate: "2022",
				Description:  "Greatest tv show ever",
				Popularity:   1,
			},
		},
	}, resp)
}

func TestSearchTVShowsUnicode(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
		require.Equal(t, "Лост", query)
		require.Equal(t, 1, len(urlOptions))
		require.Equal(t, ruLangTag, urlOptions["language"])

		return &tmdb.SearchTVShows{
			SearchTVShowsResults: &tmdb.SearchTVShowsResults{
				Results: []tmdb.TVShowResult{
					{
						ID:           4815162342,
						Name:         "Лост",
						VoteMetrics:  tmdb.VoteMetrics{VoteAverage: 9.9},
						PosterPath:   "/lost.jpg",
						FirstAirDate: "2022",
						Overview:     "Greatest tv show ever",
						Popularity:   1000,
					},
				},
			},
		}, nil
	})

	resp, err := client.client.SearchTVShows(context.Background(), "Лост")
	require.Equal(t, 1, len(client.mockedTMDB.GetSearchTVShowMock.Calls()))
	require.NoError(t, err)

	// sorted by popularity desc
	require.Equal(t, &TVShows{
		Language: ruLangTag,
		TVShows: []*TVShow{
			{
				ID:           4815162342,
				Name:         "Лост",
				Rating:       9.9,
				PosterLink:   "/img/lost.jpg",
				FirstAirDate: "2022",
				Description:  "Greatest tv show ever",
				Popularity:   1000,
			},
		},
	}, resp)
}

func TestSearchTVShowsError(t *testing.T) {
	client := NewClientM(t)
	someError := errors.New("some error")

	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
		require.Equal(t, "Lost", query)
		require.Equal(t, 1, len(urlOptions))
		require.Equal(t, enLangTag, urlOptions["language"])

		return nil, someError
	})

	resp, err := client.client.SearchTVShows(context.Background(), "Lost")

	require.Equal(t, 1, len(client.mockedTMDB.GetSearchTVShowMock.Calls()))
	require.ErrorIs(t, err, someError)
	require.Equal(t, (*TVShows)(nil), resp)
}

func TestSearchTVShowsNilResp(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
		require.Equal(t, "Lost", query)
		require.Equal(t, 1, len(urlOptions))
		require.Equal(t, enLangTag, urlOptions["language"])

		return nil, nil
	})

	resp, err := client.client.SearchTVShows(context.Background(), "Lost")

	require.Equal(t, 1, len(client.mockedTMDB.GetSearchTVShowMock.Calls()))
	require.ErrorIs(t, err, ErrNilResp)
	require.Equal(t, (*TVShows)(nil), resp)
}

func TestSearchTVShowsNilTVShows(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
		require.Equal(t, "Lost", query)
		require.Equal(t, 1, len(urlOptions))
		require.Equal(t, enLangTag, urlOptions["language"])

		return &tmdb.SearchTVShows{
			SearchTVShowsResults: nil,
		}, nil
	})

	resp, err := client.client.SearchTVShows(context.Background(), "Lost")

	require.Equal(t, 1, len(client.mockedTMDB.GetSearchTVShowMock.Calls()))
	require.ErrorIs(t, err, ErrNilResp)
	require.Equal(t, (*TVShows)(nil), resp)
}

func TestSearchTVShowsNilResults(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
		require.Equal(t, "Lost", query)
		require.Equal(t, 1, len(urlOptions))
		require.Equal(t, enLangTag, urlOptions["language"])

		return &tmdb.SearchTVShows{
			SearchTVShowsResults: &tmdb.SearchTVShowsResults{
				Results: nil,
			},
		}, nil
	})

	resp, err := client.client.SearchTVShows(context.Background(), "Lost")

	require.Equal(t, 1, len(client.mockedTMDB.GetSearchTVShowMock.Calls()))
	require.ErrorIs(t, err, ErrNilResp)
	require.Equal(t, (*TVShows)(nil), resp)
}

// TestSearchTVShowsCacheHitsOnce verifies the per-(query, lang) search cache:
// repeated calls with the same query issue exactly one TMDB request, while
// the per-call ratings override still runs every time.
func TestSearchTVShowsCacheHitsOnce(t *testing.T) {
	client := NewClientMCfg(t, searchCacheCfg())
	client.mockedTMDB.GetSearchTVShowMock.Return(&tmdb.SearchTVShows{
		SearchTVShowsResults: &tmdb.SearchTVShowsResults{
			Results: []tmdb.TVShowResult{
				{ID: 1, Name: "Lost", VoteMetrics: tmdb.VoteMetrics{VoteAverage: 7.5}, Popularity: 100},
			},
		},
	}, nil)

	// Ratings provider: ready, but never resolves an IMDb ID, so override is
	// a no-op past the Ready() gate. This isolates the cache-hit assertion
	// from any external_ids fan-out.
	client.mockedRatings.ReadyMock.Return(true)
	client.mockedTMDB.GetTVExternalIDsMock.Return(&tmdb.TVExternalIDs{IMDbID: ""}, nil)

	for range 5 {
		resp, err := client.client.SearchTVShows(context.Background(), "Lost")
		require.NoError(t, err)
		require.Len(t, resp.TVShows, 1)
		require.Equal(t, "Lost", resp.TVShows[0].Name)
	}

	require.Equal(t, uint64(1), client.mockedTMDB.GetSearchTVShowAfterCounter(),
		"cache must collapse repeated lookups to a single TMDB search call")
}

// TestSearchTVShowsCacheRatingsRefreshOnHit verifies that the override runs on
// every call against a per-call deep copy of the cached *TVShows: when the
// ratings provider's response changes between two calls, the second response
// reflects the new rating even though TMDB was hit only once.
func TestSearchTVShowsCacheRatingsRefreshOnHit(t *testing.T) {
	client := NewClientMCfg(t, searchCacheCfg())
	const tmdbRating float32 = 7.5
	client.mockedTMDB.GetSearchTVShowMock.Return(&tmdb.SearchTVShows{
		SearchTVShowsResults: &tmdb.SearchTVShowsResults{
			Results: []tmdb.TVShowResult{
				{ID: 1399, Name: "GoT", VoteMetrics: tmdb.VoteMetrics{VoteAverage: tmdbRating}, Popularity: 100},
			},
		},
	}, nil)
	client.mockedRatings.ReadyMock.Return(true)
	client.mockedTMDB.GetTVExternalIDsMock.Return(&tmdb.TVExternalIDs{IMDbID: fakeIMDbID}, nil)

	// First call: provider returns 9.1.
	client.mockedRatings.SeriesRatingMock.Return(9.1, 50000, true)
	resp1, err := client.client.SearchTVShows(context.Background(), "GoT")
	require.NoError(t, err)
	require.Len(t, resp1.TVShows, 1)
	assert.InDelta(t, 9.1, resp1.TVShows[0].Rating, 0.001)

	// Second call: provider returns 8.2 — caller must see 8.2, not the leaked
	// 9.1 from the first call's override and not the original 7.5 from TMDB.
	client.mockedRatings.SeriesRatingMock.Return(8.2, 60000, true)
	resp2, err := client.client.SearchTVShows(context.Background(), "GoT")
	require.NoError(t, err)
	require.Len(t, resp2.TVShows, 1)
	assert.InDelta(t, 8.2, resp2.TVShows[0].Rating, 0.001,
		"second call must reflect the latest provider rating, not a leaked first-call override")

	// Distinct outer + inner pointers — proves the deep copy actually happened.
	assert.NotSame(t, resp1, resp2,
		"cached *TVShows must be deep-copied per call")
	assert.NotSame(t, resp1.TVShows[0], resp2.TVShows[0],
		"cached *TVShow pointer must be deep-copied per call")

	// And exactly one TMDB search call total — the cache really did serve
	// the second request.
	require.Equal(t, uint64(1), client.mockedTMDB.GetSearchTVShowAfterCounter(),
		"second call must be served from the search cache")
}

// TestSearchTVShowsCacheKeyIsolation verifies that distinct query strings (and
// distinct resolved language tags via the unicode router) do not collide.
func TestSearchTVShowsCacheKeyIsolation(t *testing.T) {
	client := NewClientMCfg(t, searchCacheCfg())
	var calls atomic.Int32
	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, _ map[string]string) (*tmdb.SearchTVShows, error) {
		calls.Add(1)
		return &tmdb.SearchTVShows{
			SearchTVShowsResults: &tmdb.SearchTVShowsResults{
				Results: []tmdb.TVShowResult{
					{ID: 1, Name: query, Popularity: 100},
				},
			},
		}, nil
	})

	// Three distinct keys: two ASCII queries (both resolve to "en"), one
	// Cyrillic query (resolves to "ru"). All three must fetch independently.
	r1, err := client.client.SearchTVShows(context.Background(), "Lost")
	require.NoError(t, err)
	require.Equal(t, "Lost", r1.TVShows[0].Name)

	r2, err := client.client.SearchTVShows(context.Background(), "GoT")
	require.NoError(t, err)
	require.Equal(t, "GoT", r2.TVShows[0].Name)

	r3, err := client.client.SearchTVShows(context.Background(), "Лост")
	require.NoError(t, err)
	require.Equal(t, "Лост", r3.TVShows[0].Name)

	// Repeats hit the cache.
	for range 3 {
		_, err = client.client.SearchTVShows(context.Background(), "Lost")
		require.NoError(t, err)
		_, err = client.client.SearchTVShows(context.Background(), "GoT")
		require.NoError(t, err)
		_, err = client.client.SearchTVShows(context.Background(), "Лост")
		require.NoError(t, err)
	}

	require.Equal(t, int32(3), calls.Load(),
		"distinct query keys must each fetch exactly once")
}

// TestSearchTVShowsCacheErrorNotCached verifies that a TMDB error is returned
// to the caller and NOT cached: the next call re-issues the underlying request.
func TestSearchTVShowsCacheErrorNotCached(t *testing.T) {
	client := NewClientMCfg(t, searchCacheCfg())
	someError := errors.New("some error")
	client.mockedTMDB.GetSearchTVShowMock.Return(nil, someError)

	for range 2 {
		resp, err := client.client.SearchTVShows(context.Background(), "Lost")
		require.ErrorIs(t, err, someError)
		require.Nil(t, resp)
	}

	require.Equal(t, uint64(2), client.mockedTMDB.GetSearchTVShowAfterCounter(),
		"errors must not be cached; both calls must hit TMDB")
}
