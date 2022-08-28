package tvmeta

import (
	"context"
	"errors"
	"testing"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/stretchr/testify/require"
)

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
						VoteAverage:   9.19,
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
						VoteAverage:   9.29,
					},
					{
						EpisodeNumber: 2,
						Name:          "S2 Second One",
						Overview:      "S2 Greatest episode ever 2",
						VoteAverage:   9.229,
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
						VoteAverage:   9.39,
					},
				},
			}, nil
		}
		return nil, errors.New("some error")
	})

	resp, err := client.client.TVShowAllSeasonsWithDetails(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, 1, len(client.mockedTMDB.GetTVDetailsMock.Calls()))
	require.Equal(t, 3, len(client.mockedTMDB.GetTVSeasonDetailsMock.Calls()))

	require.Equal(t, &AllSeasonsWithDetails{
		Details: &TvShowDetails{
			ID:         42,
			Title:      "Lost",
			SeasonsCnt: 3,
			PosterLink: "https://image.tmdb.org/t/p/w92/lost.png",
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

	resp, err := client.client.TVShowAllSeasonsWithDetails(context.Background(), 42)
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
						VoteAverage:   9.19,
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
						VoteAverage:   9.39,
					},
				},
			}, nil
		}
		return nil, errors.New("some error")
	})

	resp, err := client.client.TVShowAllSeasonsWithDetails(context.Background(), 42)
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, 1, len(client.mockedTMDB.GetTVDetailsMock.Calls()))
	require.Equal(t, 3, len(client.mockedTMDB.GetTVSeasonDetailsMock.Calls()))

	require.Equal(t, (*AllSeasonsWithDetails)(nil), resp)
}
