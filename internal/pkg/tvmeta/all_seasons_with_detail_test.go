package tvmeta

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/stretchr/testify/require"
)

// allSeasonsCacheCfg returns a CacheConfig with the all-seasons cache enabled
// at a generous size and TTL — large enough that no test-grade timing flake can
// expire entries mid-test. Details-cache knobs are also populated so the inner
// TVShowDetails path caches deterministically alongside the wrapping
// allSeasonsCache.
func allSeasonsCacheCfg() CacheConfig {
	return CacheConfig{
		AllSeasonsSize: 64,
		AllSeasonsTTL:  time.Minute,
		DetailsSize:    64,
		DetailsTTL:     time.Minute,
	}
}

func TestTVShowAllSeasonsWithDetails(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetTVDetailsMock.Set(func(id int, urlOptions map[string]string) (tp1 *tmdb.TVDetails, err error) {
		require.Equal(t, 42, id)
		return &tmdb.TVDetails{
			Name:            "Lost",
			NumberOfSeasons: 3,
			PosterPath:      "/lost.png",
		}, nil
	})

	client.mockedTMDB.GetTVSeasonDetailsMock.Set(func(id, seasonNumber int, urlOptions map[string]string) (tp1 *tmdb.TVSeasonDetails, err error) {
		require.Equal(t, 42, id)
		switch seasonNumber {
		case 1:
			return &tmdb.TVSeasonDetails{
				Episodes: []tmdbEpisode{
					{
						EpisodeNumber: 1,
						Name:          "S1 First One",
						Overview:      "S1 Greatest episode ever",
						VoteMetrics:   tmdb.VoteMetrics{VoteAverage: 9.19},
					},
				},
			}, nil
		case 2:
			return &tmdb.TVSeasonDetails{
				Episodes: []tmdbEpisode{
					{
						EpisodeNumber: 1,
						Name:          "S2 First One",
						Overview:      "S2 Greatest episode ever",
						VoteMetrics:   tmdb.VoteMetrics{VoteAverage: 9.29},
					},
					{
						EpisodeNumber: 2,
						Name:          "S2 Second One",
						Overview:      "S2 Greatest episode ever 2",
						VoteMetrics:   tmdb.VoteMetrics{VoteAverage: 9.229},
					},
				},
			}, nil
		case 3:
			return &tmdb.TVSeasonDetails{
				Episodes: []tmdbEpisode{
					{
						EpisodeNumber: 1,
						Name:          "S3 First One",
						Overview:      "S3 Greatest episode ever",
						VoteMetrics:   tmdb.VoteMetrics{VoteAverage: 9.39},
					},
				},
			}, nil
		}
		return nil, errors.New("some error")
	})

	resp, err := client.client.TVShowAllSeasonsWithDetails(context.Background(), 42, defaultLangTag)
	require.NoError(t, err)
	require.Equal(t, 1, len(client.mockedTMDB.GetTVDetailsMock.Calls()))
	require.Equal(t, 3, len(client.mockedTMDB.GetTVSeasonDetailsMock.Calls()))

	require.Equal(t, &AllSeasonsWithDetails{
		Details: &TvShowDetails{
			ID:         42,
			Title:      "Lost",
			SeasonsCnt: 3,
			PosterLink: "/img/lost.png",
		},
		Seasons: []*TVShowSeasonEpisodes{
			{
				SeasonNumber: 1,
				Episodes: []*TVShowEpisode{
					{
						Number:      1,
						Name:        "S1 First One",
						Description: "S1 Greatest episode ever",
						Rating:      9.19,
					},
				},
			},
			{
				SeasonNumber: 2,
				Episodes: []*TVShowEpisode{
					{
						Number:      1,
						Name:        "S2 First One",
						Description: "S2 Greatest episode ever",
						Rating:      9.29,
					},
					{
						Number:      2,
						Name:        "S2 Second One",
						Description: "S2 Greatest episode ever 2",
						Rating:      9.229,
					},
				},
			},
			{
				SeasonNumber: 3,
				Episodes: []*TVShowEpisode{
					{
						Number:      1,
						Name:        "S3 First One",
						Description: "S3 Greatest episode ever",
						Rating:      9.39,
					},
				},
			},
		},
	}, resp)
}

func TestTVShowAllSeasonsWithDetailsErrorDetails(t *testing.T) {
	client := NewClientM(t)
	wantErr := errors.New("some error")

	client.mockedTMDB.GetTVDetailsMock.Set(func(id int, urlOptions map[string]string) (tp1 *tmdb.TVDetails, err error) {
		require.Equal(t, 42, id)
		return nil, wantErr
	})

	resp, err := client.client.TVShowAllSeasonsWithDetails(context.Background(), 42, defaultLangTag)
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, 1, len(client.mockedTMDB.GetTVDetailsMock.Calls()))
	require.Equal(t, 0, len(client.mockedTMDB.GetTVSeasonDetailsMock.Calls()))

	require.Equal(t, (*AllSeasonsWithDetails)(nil), resp)
}

func TestTVShowAllSeasonsWithDetailsErrorSeasonDetails(t *testing.T) {
	client := NewClientM(t)
	wantErr := errors.New("some error")

	client.mockedTMDB.GetTVDetailsMock.Set(func(id int, urlOptions map[string]string) (tp1 *tmdb.TVDetails, err error) {
		require.Equal(t, 42, id)
		return &tmdb.TVDetails{
			Name:            "Lost",
			NumberOfSeasons: 3,
			PosterPath:      "/lost.png",
		}, nil
	})

	client.mockedTMDB.GetTVSeasonDetailsMock.Set(func(id, seasonNumber int, urlOptions map[string]string) (tp1 *tmdb.TVSeasonDetails, err error) {
		require.Equal(t, 42, id)
		switch seasonNumber {
		case 1:
			return &tmdb.TVSeasonDetails{
				Episodes: []tmdbEpisode{
					{
						EpisodeNumber: 1,
						Name:          "S1 First One",
						Overview:      "S1 Greatest episode ever",
						VoteMetrics:   tmdb.VoteMetrics{VoteAverage: 9.19},
					},
				},
			}, nil
		case 2:
			return nil, wantErr
		case 3:
			return &tmdb.TVSeasonDetails{
				Episodes: []tmdbEpisode{
					{
						EpisodeNumber: 1,
						Name:          "S3 First One",
						Overview:      "S3 Greatest episode ever",
						VoteMetrics:   tmdb.VoteMetrics{VoteAverage: 9.39},
					},
				},
			}, nil
		}
		return nil, errors.New("some error")
	})

	resp, err := client.client.TVShowAllSeasonsWithDetails(context.Background(), 42, defaultLangTag)
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, 1, len(client.mockedTMDB.GetTVDetailsMock.Calls()))
	require.Equal(t, 3, len(client.mockedTMDB.GetTVSeasonDetailsMock.Calls()))

	require.Equal(t, (*AllSeasonsWithDetails)(nil), resp)
}

// TestTVShowAllSeasonsWithDetailsCacheHitsOnce verifies that two consecutive
// calls with the same (id, lang) issue exactly one GetTVDetails and one round
// of GetTVSeasonDetails calls, and that the second call skips the IMDb
// override entirely (no per-call EpisodeRating / GetTVExternalIDs traffic).
func TestTVShowAllSeasonsWithDetailsCacheHitsOnce(t *testing.T) {
	client := NewClientMCfg(t, allSeasonsCacheCfg())

	client.mockedTMDB.GetTVDetailsMock.Return(&tmdb.TVDetails{
		Name:            "Lost",
		NumberOfSeasons: 2,
	}, nil)
	client.mockedTMDB.GetTVSeasonDetailsMock.Set(func(_, seasonNumber int, _ map[string]string) (*tmdb.TVSeasonDetails, error) {
		return &tmdb.TVSeasonDetails{
			Episodes: []tmdbEpisode{
				{EpisodeNumber: 1, Name: "ep1", VoteMetrics: tmdb.VoteMetrics{VoteAverage: float32(seasonNumber) + 0.1}},
				{EpisodeNumber: 2, Name: "ep2", VoteMetrics: tmdb.VoteMetrics{VoteAverage: float32(seasonNumber) + 0.2}},
			},
		}, nil
	})

	// Ratings provider: ready, with a real IMDb mapping, so the override loop
	// actually walks every episode on a miss. EpisodeRating returns ok=false
	// to keep the assertion focused on call counts rather than rating values.
	client.mockedRatings.ReadyMock.Return(true)
	client.mockedRatings.EpisodeRatingMock.Return(0, 0, false)
	client.mockedTMDB.GetTVExternalIDsMock.Return(&tmdb.TVExternalIDs{IMDbID: fakeIMDbID}, nil)

	// First call: miss. Fan-out runs, override loop runs.
	_, err := client.client.TVShowAllSeasonsWithDetails(context.Background(), 42, enLangTag)
	require.NoError(t, err)

	tvDetailsCalls := client.mockedTMDB.GetTVDetailsAfterCounter()
	seasonDetailsCalls := client.mockedTMDB.GetTVSeasonDetailsAfterCounter()
	episodeRatingCalls := client.mockedRatings.EpisodeRatingAfterCounter()
	externalIDsCalls := client.mockedTMDB.GetTVExternalIDsAfterCounter()

	require.Equal(t, uint64(1), tvDetailsCalls, "first call must fetch TV details once")
	require.Equal(t, uint64(2), seasonDetailsCalls, "first call must fetch every season once")
	require.Equal(t, uint64(4), episodeRatingCalls, "first call must walk the override loop for every (season, episode)")
	require.Equal(t, uint64(1), externalIDsCalls, "first call must resolve the series IMDb id exactly once")

	// Second call: warm hit. No new TMDB or override traffic at all.
	_, err = client.client.TVShowAllSeasonsWithDetails(context.Background(), 42, enLangTag)
	require.NoError(t, err)

	require.Equal(t, tvDetailsCalls, client.mockedTMDB.GetTVDetailsAfterCounter(),
		"warm hit must not re-fetch TV details")
	require.Equal(t, seasonDetailsCalls, client.mockedTMDB.GetTVSeasonDetailsAfterCounter(),
		"warm hit must not re-fetch any season")
	require.Equal(t, episodeRatingCalls, client.mockedRatings.EpisodeRatingAfterCounter(),
		"warm hit must skip the override loop entirely")
	require.Equal(t, externalIDsCalls, client.mockedTMDB.GetTVExternalIDsAfterCounter(),
		"warm hit must not re-resolve the series IMDb id")
}

// TestTVShowAllSeasonsWithDetailsCacheKeyIsolation verifies that distinct
// (id, lang) tuples each fetch independently and that subsequent identical
// lookups hit the cache.
func TestTVShowAllSeasonsWithDetailsCacheKeyIsolation(t *testing.T) {
	client := NewClientMCfg(t, allSeasonsCacheCfg())

	var detailsCalls atomic.Int32
	client.mockedTMDB.GetTVDetailsMock.Set(func(_ int, _ map[string]string) (*tmdb.TVDetails, error) {
		detailsCalls.Add(1)
		return &tmdb.TVDetails{Name: "Lost", NumberOfSeasons: 1}, nil
	})
	client.mockedTMDB.GetTVSeasonDetailsMock.Return(&tmdb.TVSeasonDetails{
		Episodes: []tmdbEpisode{
			{EpisodeNumber: 1, Name: "ep1", VoteMetrics: tmdb.VoteMetrics{VoteAverage: 8.0}},
		},
	}, nil)

	// Three distinct keys: same id with two languages, plus a different id.
	_, err := client.client.TVShowAllSeasonsWithDetails(context.Background(), 42, enLangTag)
	require.NoError(t, err)
	_, err = client.client.TVShowAllSeasonsWithDetails(context.Background(), 42, ruLangTag)
	require.NoError(t, err)
	_, err = client.client.TVShowAllSeasonsWithDetails(context.Background(), 99, enLangTag)
	require.NoError(t, err)

	require.Equal(t, int32(3), detailsCalls.Load(),
		"distinct (id, lang) tuples must each fetch independently")

	// Repeats hit the cache.
	for range 3 {
		_, err = client.client.TVShowAllSeasonsWithDetails(context.Background(), 42, enLangTag)
		require.NoError(t, err)
		_, err = client.client.TVShowAllSeasonsWithDetails(context.Background(), 42, ruLangTag)
		require.NoError(t, err)
		_, err = client.client.TVShowAllSeasonsWithDetails(context.Background(), 99, enLangTag)
		require.NoError(t, err)
	}

	require.Equal(t, int32(3), detailsCalls.Load(),
		"warm repeats of any cached key must not re-fetch")
}

// TestTVShowAllSeasonsWithDetailsCacheErrorNotCached verifies that a TMDB
// error during the season fan-out is not cached — a follow-up call re-issues
// the underlying calls.
func TestTVShowAllSeasonsWithDetailsCacheErrorNotCached(t *testing.T) {
	client := NewClientMCfg(t, allSeasonsCacheCfg())
	wantErr := errors.New("some error")

	client.mockedTMDB.GetTVDetailsMock.Return(&tmdb.TVDetails{
		Name:            "Lost",
		NumberOfSeasons: 1,
	}, nil)
	client.mockedTMDB.GetTVSeasonDetailsMock.Return(nil, wantErr)

	for range 2 {
		resp, err := client.client.TVShowAllSeasonsWithDetails(context.Background(), 42, enLangTag)
		require.ErrorIs(t, err, wantErr)
		require.Nil(t, resp)
	}

	require.Equal(t, uint64(2), client.mockedTMDB.GetTVSeasonDetailsAfterCounter(),
		"errors must not be cached; both calls must re-issue the failing season fetch")
}
