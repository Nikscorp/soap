package tvmeta

import (
	"context"
	"errors"
	"testing"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allSeasonsCacheCfg enables both the details and episodes caches at sizes
// generous enough to keep tests deterministic.
func allSeasonsCacheCfg() CacheConfig {
	return CacheConfig{
		DetailsSize:  64,
		DetailsTTL:   detailsCacheCfg().DetailsTTL,
		EpisodesSize: 64,
		EpisodesTTL:  episodesCacheCfg().EpisodesTTL,
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

// TestTVShowAllSeasonsWithDetailsCachedEpisodesIsolatedFromOverride verifies
// that two consecutive calls to TVShowAllSeasonsWithDetails see independent
// IMDb rating overrides even when the underlying TMDB season-details call is
// served from the episodes cache. The cached *TVShowSeasonEpisodes must not
// carry the previous call's overridden ratings, otherwise a single
// per-request mutation leaks into shared state.
func TestTVShowAllSeasonsWithDetailsCachedEpisodesIsolatedFromOverride(t *testing.T) {
	client := NewClientMCfg(t, allSeasonsCacheCfg())
	client.mockedRatings.ReadyMock.Return(true)

	client.mockedTMDB.GetTVDetailsMock.Return(&tmdb.TVDetails{
		Name: "GoT", NumberOfSeasons: 1,
	}, nil)

	const tmdbRating float32 = 7.5
	client.mockedTMDB.GetTVSeasonDetailsMock.Return(&tmdb.TVSeasonDetails{
		Episodes: []tmdbEpisode{
			{EpisodeNumber: 1, Name: "S1E1", VoteMetrics: tmdb.VoteMetrics{VoteAverage: tmdbRating}},
		},
	}, nil)

	client.mockedTMDB.GetTVExternalIDsMock.Return(&tmdb.TVExternalIDs{IMDbID: fakeIMDbID}, nil)

	// First call: ratings provider returns 9.1 — caller must see 9.1.
	client.mockedRatings.EpisodeRatingMock.Return(9.1, 50000, true)
	resp1, err := client.client.TVShowAllSeasonsWithDetails(context.Background(), 1399, "en")
	require.NoError(t, err)
	require.Len(t, resp1.Seasons, 1)
	assert.InDelta(t, 9.1, resp1.Seasons[0].Episodes[0].Rating, 0.001)

	// Second call: ratings provider returns 8.2 — caller must see 8.2, not the
	// 9.1 that the first call wrote, and not the original 7.5 from TMDB.
	client.mockedRatings.EpisodeRatingMock.Return(8.2, 60000, true)
	resp2, err := client.client.TVShowAllSeasonsWithDetails(context.Background(), 1399, "en")
	require.NoError(t, err)
	require.Len(t, resp2.Seasons, 1)
	assert.InDelta(t, 8.2, resp2.Seasons[0].Episodes[0].Rating, 0.001,
		"second call must reflect the latest provider rating, not a leaked first-call override")

	// Distinct slice/episode pointers — proves the deep copy actually happened.
	assert.NotSame(t, resp1.Seasons[0], resp2.Seasons[0],
		"cached season pointer must be deep-copied per call")
	assert.NotSame(t, resp1.Seasons[0].Episodes[0], resp2.Seasons[0].Episodes[0],
		"cached episode pointer must be deep-copied per call")

	// And one TMDB season-details call total — the cache really did serve the
	// second request.
	require.Equal(t, uint64(1), client.mockedTMDB.GetTVSeasonDetailsAfterCounter(),
		"second call must be served from the episodes cache")
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
