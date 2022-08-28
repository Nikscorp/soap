package lazysoap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/stretchr/testify/require"
)

func TestIDHandler(t *testing.T) {
	srv := NewServerM(t)
	defer srv.server.Close()

	srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
		require.Equal(t, 42, id)
		return &tvmeta.AllSeasonsWithDetails{
			Details: &tvmeta.TvShowDetails{
				ID:         42,
				Title:      "Lost",
				PosterLink: "https://image.tmdb.org/t/p/w92/lost.png",
				SeasonsCnt: 3,
			},
			Seasons: []*tvmeta.TVShowSeasonEpisodes{
				{
					SeasonNumber: 1,
					Episodes: []*tvmeta.TVShowEpisode{
						{
							Number:      1,
							Name:        "First One",
							Description: "Greatest episode ever",
							Rating:      9,
						},
						{
							Number:      2,
							Name:        "Second One",
							Description: "Greatest episode ever2",
							Rating:      9,
						},
						{
							Number:      3,
							Name:        "Third One",
							Description: "Not so great episode",
							Rating:      1,
						},
					},
				},
				{
					SeasonNumber: 2,
					Episodes: []*tvmeta.TVShowEpisode{
						{
							Number:      1,
							Name:        "S2 First One",
							Description: "Greatest episode ever",
							Rating:      9,
						},
						{
							Number:      2,
							Name:        "S2 Second One",
							Description: "Greatest episode ever2",
							Rating:      9,
						},
						{
							Number:      3,
							Name:        "S2 Third One",
							Description: "Not so great episode",
							Rating:      1,
						},
					},
				},
				{
					SeasonNumber: 3,
					Episodes: []*tvmeta.TVShowEpisode{
						{
							Number:      1,
							Name:        "S3 First One",
							Description: "Greatest episode ever",
							Rating:      9,
						},
						{
							Number:      2,
							Name:        "S3 Second One",
							Description: "Greatest episode ever2",
							Rating:      9,
						},
						{
							Number:      3,
							Name:        "S3 Third One",
							Description: "Not so great episode",
							Rating:      1,
						},
					},
				},
			},
		}, nil
	})

	resp, err := http.Get(srv.server.URL + "/id/42")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.JSONEq(t, `
	{
		"Episodes": [
			{
				"Title": "First One",
				"imdbRating": "9.0",
				"Episode": "1",
				"Season": "1"
			},
			{
				"Title": "Second One",
				"imdbRating": "9.0",
				"Episode": "2",
				"Season": "1"
			},
			{
				"Title": "S2 First One",
				"imdbRating": "9.0",
				"Episode": "1",
				"Season": "2"
			},
			{
				"Title": "S2 Second One",
				"imdbRating": "9.0",
				"Episode": "2",
				"Season": "2"
			},
			{
				"Title": "S3 First One",
				"imdbRating": "9.0",
				"Episode": "1",
				"Season": "3"
			},
			{
				"Title": "S3 Second One",
				"imdbRating": "9.0",
				"Episode": "2",
				"Season": "3"
			}
		],
		"Title": "Lost",
		"Poster": "https://image.tmdb.org/t/p/w92/lost.png"
	}
	`, string(body))
	require.Equal(t, 1, len(srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Calls()))
}

func TestIDHandlerInvalidID(t *testing.T) {
	srv := NewServerM(t)
	defer srv.server.Close()

	resp, err := http.Get(srv.server.URL + "/id/bla")

	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "", string(body))
	require.Equal(t, 0, len(srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Calls()))
}

func TestIDHandlerTVShowEpisodesBySeasonError(t *testing.T) {
	srv := NewServerM(t)
	defer srv.server.Close()

	srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
		require.Equal(t, 42, id)
		return nil, errors.New("some error")
	})

	resp, err := http.Get(srv.server.URL + "/id/42")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "", string(body))
	require.Equal(t, 1, len(srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Calls()))
}

func TestIDHandlerZeroEpisodes(t *testing.T) {
	srv := NewServerM(t)
	defer srv.server.Close()

	srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
		require.Equal(t, 42, id)
		return &tvmeta.AllSeasonsWithDetails{
			Details: &tvmeta.TvShowDetails{
				ID:         42,
				Title:      "Lost",
				PosterLink: "https://image.tmdb.org/t/p/w92/lost.png",
				SeasonsCnt: 3,
			},
			Seasons: []*tvmeta.TVShowSeasonEpisodes{
				{
					SeasonNumber: 1,
					Episodes:     []*tvmeta.TVShowEpisode{},
				},
				{
					SeasonNumber: 2,
					Episodes:     []*tvmeta.TVShowEpisode{},
				},
				{
					SeasonNumber: 3,
					Episodes:     []*tvmeta.TVShowEpisode{},
				},
			},
		}, nil
	})
	resp, err := http.Get(srv.server.URL + "/id/42")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "", string(body))
	require.Equal(t, 1, len(srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Calls()))
}
