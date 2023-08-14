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

	testCases := []struct {
		name       string
		inputQuery string
		wantLang   string
	}{
		{
			name:       "default",
			inputQuery: "/id/42",
			wantLang:   "",
		},
		{
			name:       "enLangTag",
			inputQuery: "/id/42?language=en",
			wantLang:   "en",
		},
		{
			name:       "ruLangTag",
			inputQuery: "/id/42?language=ru",
			wantLang:   "ru",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int, language string) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
				require.Equal(t, 42, id)
				require.Equal(t, tc.wantLang, language)
				return &tvmeta.AllSeasonsWithDetails{
					Details: &tvmeta.TvShowDetails{
						ID:         42,
						Title:      "Lost",
						PosterLink: "/img/lost.png",
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
									Rating:      9.9898989898,
								},
								{
									Number:      2,
									Name:        "Second One",
									Description: "Greatest episode ever2",
									Rating:      9.8,
								},
								{
									Number:      3,
									Name:        "Third One",
									Description: "Not so great episode",
									Rating:      1.1,
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

			resp, err := http.Get(srv.server.URL + tc.inputQuery)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.JSONEq(t, `
			{
				"episodes": [
					{
						"title": "First One",
						"rating": 9.99,
						"number": 1,
						"season": 1
					},
					{
						"title": "Second One",
						"rating": 9.8,
						"number": 2,
						"season": 1
					},
					{
						"title": "S2 First One",
						"rating": 9,
						"number": 1,
						"season": 2
					},
					{
						"title": "S2 Second One",
						"rating": 9,
						"number": 2,
						"season": 2
					},
					{
						"title": "S3 First One",
						"rating": 9,
						"number": 1,
						"season": 3
					},
					{
						"title": "S3 Second One",
						"rating": 9,
						"number": 2,
						"season": 3
					}
				],
				"title": "Lost",
				"poster": "/img/lost.png"
			}
			`, string(body))
		})
	}
	require.Equal(t, len(testCases), len(srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Calls()))
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

	srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int, language string) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
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

	srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int, language string) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
		require.Equal(t, 42, id)
		return &tvmeta.AllSeasonsWithDetails{
			Details: &tvmeta.TvShowDetails{
				ID:         42,
				Title:      "Lost",
				PosterLink: "/img/lost.png",
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
