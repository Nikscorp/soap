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

// fixtureSeasons returns the canonical 9-episode/3-season fixture used by the
// /id/{id} tests. Episodes ratings (after server-side rounding) are:
// S1E1=9.99, S1E2=9.8, S1E3=1.1, S2E1=9, S2E2=9, S2E3=1, S3E1=9, S3E2=9, S3E3=1.
// avg ≈ 6.543, so 6 episodes are at or above the average ⇒ defaultBest=6.
func fixtureSeasons() *tvmeta.AllSeasonsWithDetails {
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
					{Number: 1, Name: "First One", Description: "Greatest episode ever", Rating: 9.9898989898},
					{Number: 2, Name: "Second One", Description: "Greatest episode ever2", Rating: 9.8},
					{Number: 3, Name: "Third One", Description: "Not so great episode", Rating: 1.1},
				},
			},
			{
				SeasonNumber: 2,
				Episodes: []*tvmeta.TVShowEpisode{
					{Number: 1, Name: "S2 First One", Description: "Greatest episode ever", Rating: 9},
					{Number: 2, Name: "S2 Second One", Description: "Greatest episode ever2", Rating: 9},
					{Number: 3, Name: "S2 Third One", Description: "Not so great episode", Rating: 1},
				},
			},
			{
				SeasonNumber: 3,
				Episodes: []*tvmeta.TVShowEpisode{
					{Number: 1, Name: "S3 First One", Description: "Greatest episode ever", Rating: 9},
					{Number: 2, Name: "S3 Second One", Description: "Greatest episode ever2", Rating: 9},
					{Number: 3, Name: "S3 Third One", Description: "Not so great episode", Rating: 1},
				},
			},
		},
	}
}

const (
	defaultBestBody = `
	{
		"episodes": [
			{"title": "First One",     "description": "Greatest episode ever",  "rating": 9.99, "number": 1, "season": 1},
			{"title": "Second One",    "description": "Greatest episode ever2", "rating": 9.8,  "number": 2, "season": 1},
			{"title": "S2 First One",  "description": "Greatest episode ever",  "rating": 9,    "number": 1, "season": 2},
			{"title": "S2 Second One", "description": "Greatest episode ever2", "rating": 9,    "number": 2, "season": 2},
			{"title": "S3 First One",  "description": "Greatest episode ever",  "rating": 9,    "number": 1, "season": 3},
			{"title": "S3 Second One", "description": "Greatest episode ever2", "rating": 9,    "number": 2, "season": 3}
		],
		"title": "Lost",
		"poster": "/img/lost.png",
		"defaultBest": 6,
		"totalEpisodes": 9
	}`

	limitTwoBody = `
	{
		"episodes": [
			{"title": "First One",  "description": "Greatest episode ever",  "rating": 9.99, "number": 1, "season": 1},
			{"title": "Second One", "description": "Greatest episode ever2", "rating": 9.8,  "number": 2, "season": 1}
		],
		"title": "Lost",
		"poster": "/img/lost.png",
		"defaultBest": 6,
		"totalEpisodes": 9
	}`

	allEpisodesBody = `
	{
		"episodes": [
			{"title": "First One",     "description": "Greatest episode ever",  "rating": 9.99, "number": 1, "season": 1},
			{"title": "Second One",    "description": "Greatest episode ever2", "rating": 9.8,  "number": 2, "season": 1},
			{"title": "Third One",     "description": "Not so great episode",   "rating": 1.1,  "number": 3, "season": 1},
			{"title": "S2 First One",  "description": "Greatest episode ever",  "rating": 9,    "number": 1, "season": 2},
			{"title": "S2 Second One", "description": "Greatest episode ever2", "rating": 9,    "number": 2, "season": 2},
			{"title": "S2 Third One",  "description": "Not so great episode",   "rating": 1,    "number": 3, "season": 2},
			{"title": "S3 First One",  "description": "Greatest episode ever",  "rating": 9,    "number": 1, "season": 3},
			{"title": "S3 Second One", "description": "Greatest episode ever2", "rating": 9,    "number": 2, "season": 3},
			{"title": "S3 Third One",  "description": "Not so great episode",   "rating": 1,    "number": 3, "season": 3}
		],
		"title": "Lost",
		"poster": "/img/lost.png",
		"defaultBest": 6,
		"totalEpisodes": 9
	}`
)

func TestIDHandler(t *testing.T) {
	srv := NewServerM(t)
	defer srv.server.Close()

	testCases := []struct {
		name       string
		inputQuery string
		wantLang   string
		wantBody   string
	}{
		{
			name:       "default",
			inputQuery: "/id/42",
			wantLang:   "",
			wantBody:   defaultBestBody,
		},
		{
			name:       "enLangTag",
			inputQuery: "/id/42?language=en",
			wantLang:   "en",
			wantBody:   defaultBestBody,
		},
		{
			name:       "ruLangTag",
			inputQuery: "/id/42?language=ru",
			wantLang:   "ru",
			wantBody:   defaultBestBody,
		},
		{
			name:       "limitTwo",
			inputQuery: "/id/42?limit=2",
			wantLang:   "",
			wantBody:   limitTwoBody,
		},
		{
			name:       "limitClampedToTotal",
			inputQuery: "/id/42?limit=999",
			wantLang:   "",
			wantBody:   allEpisodesBody,
		},
		{
			name:       "limitNonNumericFallsBackToDefault",
			inputQuery: "/id/42?limit=abc",
			wantLang:   "",
			wantBody:   defaultBestBody,
		},
		{
			name:       "limitNegativeFallsBackToDefault",
			inputQuery: "/id/42?limit=-3",
			wantLang:   "",
			wantBody:   defaultBestBody,
		},
		{
			name:       "limitZeroFallsBackToDefault",
			inputQuery: "/id/42?limit=0",
			wantLang:   "",
			wantBody:   defaultBestBody,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int, language string) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
				require.Equal(t, 42, id)
				require.Equal(t, tc.wantLang, language)
				return fixtureSeasons(), nil
			})

			resp, err := http.Get(srv.server.URL + tc.inputQuery)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.JSONEq(t, tc.wantBody, string(body))
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
