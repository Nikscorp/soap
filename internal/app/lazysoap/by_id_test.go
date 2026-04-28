package lazysoap

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nikscorp/soap/internal/app/lazysoap/mocks"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/stretchr/testify/require"
)

// fixtureSeasons returns the canonical 9-episode/3-season fixture used by the
// /id/{id} tests. Episodes ratings (after server-side rounding) are:
// S1E1=9.99, S1E2=9.8, S1E3=1.1, S2E1=9, S2E2=9, S2E3=1, S3E1=9, S3E2=9, S3E3=1.
// With q=0.9 the 0.9-quantile rating is 9.8 (ratings sorted asc, index
// floor(0.9*8)=7 ⇒ value 9.8); only S1E1 strictly exceeds it, so the
// minimum-episodes floor of 3 wins ⇒ defaultBest=3.
func fixtureSeasons() *tvmeta.AllSeasonsWithDetails {
	return &tvmeta.AllSeasonsWithDetails{
		Details: &tvmeta.TvShowDetails{
			ID:           42,
			Title:        "Lost",
			PosterLink:   "/img/lost.png",
			SeasonsCnt:   3,
			FirstAirDate: "2004-09-22",
			Overview:     "A plane crashes on a mysterious island.",
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
			{"title": "S2 First One",  "description": "Greatest episode ever",  "rating": 9,    "number": 1, "season": 2}
		],
		"title": "Lost",
		"poster": "/img/lost.png",
		"firstAirDate": "2004-09-22",
		"description": "A plane crashes on a mysterious island.",
		"defaultBest": 3,
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
		"firstAirDate": "2004-09-22",
		"description": "A plane crashes on a mysterious island.",
		"defaultBest": 3,
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
		"firstAirDate": "2004-09-22",
		"description": "A plane crashes on a mysterious island.",
		"defaultBest": 3,
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

func TestIDHandlerDefaultBestConfig(t *testing.T) {
	testCases := []struct {
		name            string
		quantile        float32
		minEpisodes     int
		wantDefaultBest int
	}{
		// q=0.5 over the 9-episode fixture: ratings asc index floor(0.5*8)=4
		// → threshold rating 9 (the median 9), 2 episodes strictly above (9.99, 9.8).
		{name: "median quantile picks tail above threshold", quantile: 0.5, minEpisodes: 0, wantDefaultBest: 2},
		// Same quantile but with a higher floor.
		{name: "minEpisodes raises the floor", quantile: 0.5, minEpisodes: 5, wantDefaultBest: 5},
		// q=1.0: threshold equals the highest rating, so no episode strictly
		// exceeds it; the floor wins.
		{name: "q=1 falls back to minEpisodes", quantile: 1.0, minEpisodes: 4, wantDefaultBest: 4},
		// minEpisodes greater than total clamps to total.
		{name: "minEpisodes capped at totalEpisodes", quantile: 1.0, minEpisodes: 100, wantDefaultBest: 9},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tvMetaClientMock := mocks.NewTvMetaClientMock(t)
			srv := New(Config{
				DefaultBestQuantile:    tc.quantile,
				DefaultBestMinEpisodes: tc.minEpisodes,
			}, tvMetaClientMock, "")
			ts := httptest.NewServer(srv.newRouter())
			defer ts.Close()

			tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int, language string) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
				return fixtureSeasons(), nil
			})

			resp, err := http.Get(ts.URL + "/id/42")
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)

			var got episodesResp
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
			require.Equal(t, tc.wantDefaultBest, got.DefaultBest)
			require.Equal(t, 9, got.TotalEpisodes)
			require.Len(t, got.Episodes, tc.wantDefaultBest)
		})
	}
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
