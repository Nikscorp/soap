package tvmeta

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/tvmeta/mocks"
	tmdb "github.com/cyruzin/golang-tmdb"
)

const (
	benchSeasonsCnt        = 8
	benchEpisodesPerSeason = 10
	benchSearchResults     = 20
	benchTVID              = 1399
	benchLang              = "en"
	benchIMDbID            = "tt0944947"
)

func benchCacheCfg() CacheConfig {
	return CacheConfig{
		DetailsSize:    64,
		DetailsTTL:     time.Minute,
		AllSeasonsSize: 64,
		AllSeasonsTTL:  time.Minute,
		SearchSize:     64,
		SearchTTL:      time.Minute,
	}
}

func benchSeasonResponse(season int) *tmdb.TVSeasonDetails {
	eps := make([]tmdbEpisode, benchEpisodesPerSeason)
	for i := range eps {
		eps[i] = tmdbEpisode{
			EpisodeNumber: i + 1,
			SeasonNumber:  season,
			Name:          fmt.Sprintf("S%dE%d", season, i+1),
			Overview:      "synthetic episode overview",
			VoteMetrics:   tmdb.VoteMetrics{VoteAverage: 7.5},
			StillPath:     "/still.jpg",
		}
	}
	return &tmdb.TVSeasonDetails{Episodes: eps}
}

func benchSearchResponse() *tmdb.SearchTVShows {
	results := make([]tmdb.TVShowResult, benchSearchResults)
	for i := range results {
		results[i] = tmdb.TVShowResult{
			ID:           int64(1000 + i),
			Name:         fmt.Sprintf("Show %d", i),
			VoteMetrics:  tmdb.VoteMetrics{VoteAverage: 7.5, VoteCount: 1000},
			PosterPath:   "/poster.jpg",
			FirstAirDate: "2020",
			Overview:     "synthetic search result",
			Popularity:   float32(benchSearchResults - i),
		}
	}
	return &tmdb.SearchTVShows{
		SearchTVShowsResults: &tmdb.SearchTVShowsResults{Results: results},
	}
}

func benchSetupAllSeasonsMocks(tb testing.TB, cfg CacheConfig) (*Client, *mocks.TmdbClientMock) {
	tb.Helper()
	tmdbMock := mocks.NewTmdbClientMock(tb)
	tmdbMock.GetTVDetailsMock.Return(&tmdb.TVDetails{
		Name:            "Synthetic",
		NumberOfSeasons: benchSeasonsCnt,
		PosterPath:      "/poster.jpg",
		FirstAirDate:    "2020",
	}, nil)
	tmdbMock.GetTVSeasonDetailsMock.Set(func(_, seasonNumber int, _ map[string]string) (*tmdb.TVSeasonDetails, error) {
		return benchSeasonResponse(seasonNumber), nil
	})
	tmdbMock.GetTVExternalIDsMock.Return(&tmdb.TVExternalIDs{IMDbID: benchIMDbID}, nil)

	ratings := mocks.NewRatingsProviderMock(tb)
	ratings.ReadyMock.Return(true)
	ratings.EpisodeRatingMock.Return(8.5, 50000, true)
	ratings.SeriesRatingMock.Return(8.5, 50000, true)

	return New(tmdbMock, ratings, cfg, nil), tmdbMock
}

func benchSetupSearchMocks(tb testing.TB, cfg CacheConfig) (*Client, *mocks.TmdbClientMock) {
	tb.Helper()
	tmdbMock := mocks.NewTmdbClientMock(tb)
	tmdbMock.GetSearchTVShowMock.Return(benchSearchResponse(), nil)
	tmdbMock.GetTVExternalIDsMock.Return(&tmdb.TVExternalIDs{IMDbID: benchIMDbID}, nil)

	ratings := mocks.NewRatingsProviderMock(tb)
	ratings.ReadyMock.Return(true)
	ratings.SeriesRatingMock.Return(8.5, 50000, true)
	ratings.EpisodeRatingMock.Return(8.5, 50000, true)

	return New(tmdbMock, ratings, cfg, nil), tmdbMock
}

func BenchmarkTVShowAllSeasonsWithDetails_Warm(b *testing.B) {
	cfg := benchCacheCfg()
	client, _ := benchSetupAllSeasonsMocks(b, cfg)
	ctx := context.Background()

	if _, err := client.TVShowAllSeasonsWithDetails(ctx, benchTVID, benchLang); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := client.TVShowAllSeasonsWithDetails(ctx, benchTVID, benchLang); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTVShowAllSeasonsWithDetails_Cold(b *testing.B) {
	cfg := benchCacheCfg()
	tmdbMock := mocks.NewTmdbClientMock(b)
	tmdbMock.GetTVDetailsMock.Return(&tmdb.TVDetails{
		Name:            "Synthetic",
		NumberOfSeasons: benchSeasonsCnt,
		PosterPath:      "/poster.jpg",
		FirstAirDate:    "2020",
	}, nil)
	tmdbMock.GetTVSeasonDetailsMock.Set(func(_, seasonNumber int, _ map[string]string) (*tmdb.TVSeasonDetails, error) {
		return benchSeasonResponse(seasonNumber), nil
	})
	tmdbMock.GetTVExternalIDsMock.Return(&tmdb.TVExternalIDs{IMDbID: benchIMDbID}, nil)
	ratings := mocks.NewRatingsProviderMock(b)
	ratings.ReadyMock.Return(true)
	ratings.EpisodeRatingMock.Return(8.5, 50000, true)
	ratings.SeriesRatingMock.Return(8.5, 50000, true)

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		client := New(tmdbMock, ratings, cfg, nil)
		if _, err := client.TVShowAllSeasonsWithDetails(ctx, benchTVID, benchLang); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchTVShows_Warm(b *testing.B) {
	cfg := benchCacheCfg()
	client, _ := benchSetupSearchMocks(b, cfg)
	ctx := context.Background()

	if _, err := client.SearchTVShows(ctx, "lost"); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := client.SearchTVShows(ctx, "lost"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchTVShows_Cold(b *testing.B) {
	cfg := benchCacheCfg()
	tmdbMock := mocks.NewTmdbClientMock(b)
	tmdbMock.GetSearchTVShowMock.Return(benchSearchResponse(), nil)
	tmdbMock.GetTVExternalIDsMock.Return(&tmdb.TVExternalIDs{IMDbID: benchIMDbID}, nil)
	ratings := mocks.NewRatingsProviderMock(b)
	ratings.ReadyMock.Return(true)
	ratings.SeriesRatingMock.Return(8.5, 50000, true)
	ratings.EpisodeRatingMock.Return(8.5, 50000, true)

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		client := New(tmdbMock, ratings, cfg, nil)
		if _, err := client.SearchTVShows(ctx, "lost"); err != nil {
			b.Fatal(err)
		}
	}
}
