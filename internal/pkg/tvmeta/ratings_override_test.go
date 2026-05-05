package tvmeta

import (
	"context"
	"errors"
	"testing"

	"github.com/Nikscorp/soap/internal/pkg/tvmeta/mocks"
	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fakeIMDbID = "tt0944947"

// TestAllSeasonsRatingOverridePerEpisodeFallback exercises the happy path
// (IMDb hit), the missing-entry path (IMDb miss → keep TMDB), and confirms
// the series external_ids endpoint is hit exactly once for the whole series.
func TestAllSeasonsRatingOverridePerEpisodeFallback(t *testing.T) {
	c := NewClientM(t)
	c.mockedRatings.ReadyMock.Return(true)

	c.mockedTMDB.GetTVDetailsMock.Return(&tmdb.TVDetails{
		Name:            "GoT",
		NumberOfSeasons: 1,
		PosterPath:      "/got.jpg",
	}, nil)

	c.mockedTMDB.GetTVSeasonDetailsMock.Set(func(_, seasonNumber int, _ map[string]string) (*tmdb.TVSeasonDetails, error) {
		require.Equal(t, 1, seasonNumber)
		return &tmdb.TVSeasonDetails{
			Episodes: []tmdbEpisode{
				{EpisodeNumber: 1, Name: "Winter Is Coming", VoteMetrics: tmdb.VoteMetrics{VoteAverage: 7.5}},
				{EpisodeNumber: 2, Name: "The Kingsroad", VoteMetrics: tmdb.VoteMetrics{VoteAverage: 7.4}},
				{EpisodeNumber: 3, Name: "Lord Snow", VoteMetrics: tmdb.VoteMetrics{VoteAverage: 7.3}},
			},
		}, nil
	})

	c.mockedTMDB.GetTVExternalIDsMock.Set(func(id int, _ map[string]string) (*tmdb.TVExternalIDs, error) {
		require.Equal(t, 1399, id)
		return &tmdb.TVExternalIDs{IMDbID: fakeIMDbID}, nil
	})

	// IMDb knows ep 1 and 3, but not 2 — ep 2 must keep its TMDB rating.
	c.mockedRatings.EpisodeRatingMock.Set(func(seriesIMDbID string, season, episode int) (float32, uint32, bool) {
		require.Equal(t, fakeIMDbID, seriesIMDbID)
		require.Equal(t, 1, season)
		switch episode {
		case 1:
			return 9.1, 50000, true
		case 3:
			return 8.9, 30000, true
		default:
			return 0, 0, false
		}
	})

	resp, err := c.client.TVShowAllSeasonsWithDetails(context.Background(), 1399, "en")
	require.NoError(t, err)
	require.Len(t, resp.Seasons, 1)
	eps := resp.Seasons[0].Episodes
	require.Len(t, eps, 3)

	assert.InDelta(t, 9.1, eps[0].Rating, 0.001, "ep1 should be IMDb 9.1")
	assert.InDelta(t, 7.4, eps[1].Rating, 0.001, "ep2 must fall back to TMDB 7.4")
	assert.InDelta(t, 8.9, eps[2].Rating, 0.001, "ep3 should be IMDb 8.9")

	require.Equal(t, uint64(1), c.mockedTMDB.GetTVExternalIDsAfterCounter(),
		"external_ids should be called exactly once per series")
}

// TestAllSeasonsRatingOverrideSkippedWhenNotReady ensures we don't burn an
// extra TMDB external_ids call while the dataset is still loading at boot.
func TestAllSeasonsRatingOverrideSkippedWhenNotReady(t *testing.T) {
	c := NewClientM(t)
	c.mockedRatings.ReadyMock.Return(false)

	c.mockedTMDB.GetTVDetailsMock.Return(&tmdb.TVDetails{
		Name: "x", NumberOfSeasons: 1,
	}, nil)
	c.mockedTMDB.GetTVSeasonDetailsMock.Return(&tmdb.TVSeasonDetails{
		Episodes: []tmdbEpisode{
			{EpisodeNumber: 1, VoteMetrics: tmdb.VoteMetrics{VoteAverage: 7.5}},
		},
	}, nil)

	resp, err := c.client.TVShowAllSeasonsWithDetails(context.Background(), 42, "en")
	require.NoError(t, err)
	assert.InDelta(t, 7.5, resp.Seasons[0].Episodes[0].Rating, 0.001)
	assert.Equal(t, uint64(0), c.mockedTMDB.GetTVExternalIDsAfterCounter(),
		"external_ids must NOT be called when ratings provider isn't ready")
}

// TestAllSeasonsRatingOverrideSkippedOnExternalIDsFailure verifies that a
// transient TMDB external_ids error does not corrupt the response — TMDB
// ratings are returned as-is, with no panic and no IMDb lookup attempted.
func TestAllSeasonsRatingOverrideSkippedOnExternalIDsFailure(t *testing.T) {
	c := NewClientM(t)
	c.mockedRatings.ReadyMock.Return(true)

	c.mockedTMDB.GetTVDetailsMock.Return(&tmdb.TVDetails{
		Name: "x", NumberOfSeasons: 1,
	}, nil)
	c.mockedTMDB.GetTVSeasonDetailsMock.Return(&tmdb.TVSeasonDetails{
		Episodes: []tmdbEpisode{
			{EpisodeNumber: 1, VoteMetrics: tmdb.VoteMetrics{VoteAverage: 6.6}},
		},
	}, nil)
	c.mockedTMDB.GetTVExternalIDsMock.Return(nil, errors.New("boom"))

	resp, err := c.client.TVShowAllSeasonsWithDetails(context.Background(), 42, "en")
	require.NoError(t, err)
	assert.InDelta(t, 6.6, resp.Seasons[0].Episodes[0].Rating, 0.001,
		"ratings must fall back to TMDB on external_ids failure")
}

// TestSearchTVShowsRatingOverride checks that search results get their
// per-show ratings replaced when IMDb has them, and that misses fall through
// to TMDB.
func TestSearchTVShowsRatingOverride(t *testing.T) {
	c := NewClientM(t)
	c.mockedRatings.ReadyMock.Return(true)

	c.mockedTMDB.GetSearchTVShowMock.Return(&tmdb.SearchTVShows{
		SearchTVShowsResults: &tmdb.SearchTVShowsResults{
			Results: []tmdb.TVShowResult{
				{ID: 1399, Name: "GoT", VoteMetrics: tmdb.VoteMetrics{VoteAverage: 8.4}, Popularity: 100},
				{ID: 1396, Name: "BB", VoteMetrics: tmdb.VoteMetrics{VoteAverage: 8.5}, Popularity: 90},
			},
		},
	}, nil)

	c.mockedTMDB.GetTVExternalIDsMock.Set(func(id int, _ map[string]string) (*tmdb.TVExternalIDs, error) {
		switch id {
		case 1399:
			return &tmdb.TVExternalIDs{IMDbID: "tt0944947"}, nil
		case 1396:
			return &tmdb.TVExternalIDs{IMDbID: ""}, nil // no IMDb mapping
		}
		return nil, errors.New("unexpected id")
	})

	c.mockedRatings.SeriesRatingMock.Set(func(imdbID string) (float32, uint32, bool) {
		if imdbID == "tt0944947" {
			return 9.2, 2_300_000, true
		}
		return 0, 0, false
	})

	resp, err := c.client.SearchTVShows(context.Background(), "got")
	require.NoError(t, err)
	require.Len(t, resp.TVShows, 2)

	// Sorted by popularity desc, so GoT (popularity 100) first.
	assert.Equal(t, 1399, resp.TVShows[0].ID)
	assert.InDelta(t, 9.2, resp.TVShows[0].Rating, 0.001, "GoT rating overridden by IMDb")

	assert.Equal(t, 1396, resp.TVShows[1].ID)
	assert.InDelta(t, 8.5, resp.TVShows[1].Rating, 0.001, "BB has no IMDb id, keeps TMDB rating")
}

// TestSeriesIMDbIDCacheHitsOnce verifies the in-process TMDB→IMDb-ID cache:
// repeated lookups for the same series ID issue a single external_ids call.
func TestSeriesIMDbIDCacheHitsOnce(t *testing.T) {
	tmdbClient := mocks.NewTmdbClientMock(t)
	tmdbClient.GetTVExternalIDsMock.Return(&tmdb.TVExternalIDs{IMDbID: fakeIMDbID}, nil)

	client := New(tmdbClient, NoopRatingsProvider{}, CacheConfig{}, nil)
	for range 5 {
		got := client.seriesIMDbID(context.Background(), 1399)
		assert.Equal(t, fakeIMDbID, got)
	}
	assert.Equal(t, uint64(1), tmdbClient.GetTVExternalIDsAfterCounter(),
		"cache must collapse repeated lookups to a single TMDB call")
}
