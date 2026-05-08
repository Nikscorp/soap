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
	"github.com/stretchr/testify/assert"
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
		"totalEpisodes": 9,
		"availableSeasons": [1, 2, 3]
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
		"totalEpisodes": 9,
		"availableSeasons": [1, 2, 3]
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
		"totalEpisodes": 9,
		"availableSeasons": [1, 2, 3]
	}`

	seasonsOneBody = `
	{
		"episodes": [
			{"title": "First One",  "description": "Greatest episode ever",  "rating": 9.99, "number": 1, "season": 1},
			{"title": "Second One", "description": "Greatest episode ever2", "rating": 9.8,  "number": 2, "season": 1},
			{"title": "Third One",  "description": "Not so great episode",   "rating": 1.1,  "number": 3, "season": 1}
		],
		"title": "Lost",
		"poster": "/img/lost.png",
		"firstAirDate": "2004-09-22",
		"description": "A plane crashes on a mysterious island.",
		"defaultBest": 3,
		"totalEpisodes": 3,
		"availableSeasons": [1, 2, 3]
	}`

	seasonsTwoThreeBody = `
	{
		"episodes": [
			{"title": "S2 First One",  "description": "Greatest episode ever",  "rating": 9, "number": 1, "season": 2},
			{"title": "S2 Second One", "description": "Greatest episode ever2", "rating": 9, "number": 2, "season": 2},
			{"title": "S3 First One",  "description": "Greatest episode ever",  "rating": 9, "number": 1, "season": 3}
		],
		"title": "Lost",
		"poster": "/img/lost.png",
		"firstAirDate": "2004-09-22",
		"description": "A plane crashes on a mysterious island.",
		"defaultBest": 3,
		"totalEpisodes": 6,
		"availableSeasons": [1, 2, 3]
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
		{
			name:       "seasonsAllExplicit",
			inputQuery: "/id/42?seasons=1,2,3",
			wantLang:   "",
			wantBody:   defaultBestBody,
		},
		{
			name:       "seasonsSingle",
			inputQuery: "/id/42?seasons=1",
			wantLang:   "",
			wantBody:   seasonsOneBody,
		},
		{
			name:       "seasonsSubset",
			inputQuery: "/id/42?seasons=2,3",
			wantLang:   "",
			wantBody:   seasonsTwoThreeBody,
		},
		{
			name:       "seasonsValidPlusUnknownIgnoresUnknown",
			inputQuery: "/id/42?seasons=1,99",
			wantLang:   "",
			wantBody:   seasonsOneBody,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int, language string) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
				assert.Equal(t, 42, id)
				assert.Equal(t, tc.wantLang, language)
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
			}, tvMetaClientMock, nil, "")
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

func TestParseSeasons(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		want    []int
		wantOK  bool
	}{
		{name: "absent", input: "", want: nil, wantOK: true},
		{name: "whitespaceOnly", input: "   ", want: nil, wantOK: true},
		{name: "single", input: "1", want: []int{1}, wantOK: true},
		{name: "ascending", input: "1,2,3", want: []int{1, 2, 3}, wantOK: true},
		{name: "unsortedSorted", input: "3,1,2", want: []int{1, 2, 3}, wantOK: true},
		{name: "duplicatesDeduped", input: "1,1,2", want: []int{1, 2}, wantOK: true},
		{name: "whitespaceTolerated", input: "1, 2 ,3", want: []int{1, 2, 3}, wantOK: true},
		{name: "nonNumericToken", input: "1,abc", want: nil, wantOK: false},
		{name: "zeroRejected", input: "0", want: nil, wantOK: false},
		{name: "negativeRejected", input: "-1", want: nil, wantOK: false},
		{name: "mixedZeroRejected", input: "1,0,2", want: nil, wantOK: false},
		{name: "emptyTokenRejected", input: "1,,2", want: nil, wantOK: false},
		{name: "trailingCommaRejected", input: "1,2,", want: nil, wantOK: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseSeasons(tc.input)
			require.Equal(t, tc.wantOK, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestIDHandlerInvalidID(t *testing.T) {
	for _, path := range []string{"/id/bla", "/id/0", "/id/-1"} {
		t.Run(path, func(t *testing.T) {
			srv := NewServerM(t)
			defer srv.server.Close()

			resp, err := http.Get(srv.server.URL + path)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
			require.Equal(t, "", string(body))
			require.Equal(t, 0, len(srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Calls()))
		})
	}
}

func TestIDHandlerTVShowEpisodesBySeasonError(t *testing.T) {
	srv := NewServerM(t)
	defer srv.server.Close()

	srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int, language string) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
		assert.Equal(t, 42, id)
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

func TestIDHandlerSeasonsMalformedReturns400(t *testing.T) {
	testCases := []struct {
		name  string
		query string
	}{
		{name: "nonNumeric", query: "/id/42?seasons=abc"},
		{name: "negative", query: "/id/42?seasons=-1"},
		{name: "zero", query: "/id/42?seasons=0"},
		{name: "trailingComma", query: "/id/42?seasons=1,2,"},
		{name: "emptyToken", query: "/id/42?seasons=1,,2"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := NewServerM(t)
			defer srv.server.Close()

			resp, err := http.Get(srv.server.URL + tc.query)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
			require.Equal(t, "", string(body))
			// Malformed param is rejected before TMDB is asked.
			require.Equal(t, 0, len(srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Calls()))
		})
	}
}

func TestIDHandlerSeasonsEmptyIntersectionReturns400(t *testing.T) {
	srv := NewServerM(t)
	defer srv.server.Close()

	srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int, language string) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
		assert.Equal(t, 42, id)
		return fixtureSeasons(), nil
	})

	resp, err := http.Get(srv.server.URL + "/id/42?seasons=99,100")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, "", string(body))
	require.Equal(t, 1, len(srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Calls()))
}

func TestIDHandlerSeasonsAvailableSeasonsAlwaysFull(t *testing.T) {
	srv := NewServerM(t)
	defer srv.server.Close()

	srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int, language string) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
		assert.Equal(t, 42, id)
		return fixtureSeasons(), nil
	})

	resp, err := http.Get(srv.server.URL + "/id/42?seasons=1")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got episodesResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, []int{1, 2, 3}, got.AvailableSeasons,
		"availableSeasons must reflect every season the series has, regardless of filter")
	require.Equal(t, 3, got.TotalEpisodes, "totalEpisodes must reflect filtered subset")
	for _, ep := range got.Episodes {
		require.Equal(t, 1, ep.Season, "filtered response must only contain selected seasons")
	}
}

func TestFilterSeasonsByNumberDoesNotMutateInput(t *testing.T) {
	in := fixtureSeasons()
	originalLen := len(in.Seasons)
	originalNumbers := make([]int, 0, originalLen)
	for _, s := range in.Seasons {
		originalNumbers = append(originalNumbers, s.SeasonNumber)
	}

	keep := map[int]struct{}{1: {}}
	got := filterSeasonsByNumber(in.Seasons, keep)

	require.Len(t, got, 1)
	require.Equal(t, 1, got[0].SeasonNumber)
	// Cached input must remain untouched.
	require.Equal(t, originalLen, len(in.Seasons))
	for i, s := range in.Seasons {
		require.Equal(t, originalNumbers[i], s.SeasonNumber)
	}
}

func TestFilterSeasonsByNumberEmptyKeepIsPassthrough(t *testing.T) {
	in := fixtureSeasons()
	got := filterSeasonsByNumber(in.Seasons, nil)
	require.Equal(t, in.Seasons, got, "nil keep set means no filter")

	got = filterSeasonsByNumber(in.Seasons, map[int]struct{}{})
	require.Equal(t, in.Seasons, got, "empty keep set means no filter")
}

// Empty seasons (e.g. TMDB placeholder seasons for ongoing shows) must be
// excluded from availableSeasons so the client never renders a chip that
// resolves to an empty filter. A no-filter request still returns the populated
// seasons; a filter targeting only the empty season collapses to the
// "intersection empty" 400 path.
func TestIDHandlerEmptySeasonsExcludedFromAvailable(t *testing.T) {
	fixture := func() *tvmeta.AllSeasonsWithDetails {
		return &tvmeta.AllSeasonsWithDetails{
			Details: &tvmeta.TvShowDetails{
				ID:         42,
				Title:      "Lost",
				PosterLink: "/img/lost.png",
				SeasonsCnt: 3,
			},
			Seasons: []*tvmeta.TVShowSeasonEpisodes{
				{SeasonNumber: 1, Episodes: []*tvmeta.TVShowEpisode{}},
				{
					SeasonNumber: 2,
					Episodes: []*tvmeta.TVShowEpisode{
						{Number: 1, Name: "S2E1", Rating: 9},
					},
				},
				{SeasonNumber: 3, Episodes: nil},
			},
		}
	}

	t.Run("noFilterOmitsEmptySeasons", func(t *testing.T) {
		srv := NewServerM(t)
		defer srv.server.Close()
		srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int, language string) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
			return fixture(), nil
		})

		resp, err := http.Get(srv.server.URL + "/id/42")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var got episodesResp
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		require.Equal(t, []int{2}, got.AvailableSeasons,
			"availableSeasons must skip seasons with zero episodes")
		require.Equal(t, 1, got.TotalEpisodes)
	})

	t.Run("filterTargetingEmptySeasonReturns400", func(t *testing.T) {
		srv := NewServerM(t)
		defer srv.server.Close()
		srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int, language string) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
			return fixture(), nil
		})

		resp, err := http.Get(srv.server.URL + "/id/42?seasons=1")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		require.Equal(t, "", string(body))
	})
}

func TestIDHandlerZeroEpisodes(t *testing.T) {
	srv := NewServerM(t)
	defer srv.server.Close()

	srv.tvMetaClientMock.TVShowAllSeasonsWithDetailsMock.Set(func(ctx context.Context, id int, language string) (ap1 *tvmeta.AllSeasonsWithDetails, err error) {
		assert.Equal(t, 42, id)
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

func TestQuantileRating(t *testing.T) {
	testCases := []struct {
		name string
		eps  []episode
		q    float32
		want float32
	}{
		{
			name: "single episode returns that episode's rating",
			eps:  []episode{{Rating: 7.5}},
			q:    0.9,
			want: 7.5,
		},
		{
			name: "q=0 returns the lowest rating (last in descending slice)",
			// descending: [9, 8, 7, 6, 5] → ascending index 0 → 5
			eps:  []episode{{Rating: 9}, {Rating: 8}, {Rating: 7}, {Rating: 6}, {Rating: 5}},
			q:    0,
			want: 5,
		},
		{
			name: "q=1 returns the highest rating (first in descending slice)",
			// descending: [9, 8, 7, 6, 5] → ascending index floor(1*4)=4 → 9
			eps:  []episode{{Rating: 9}, {Rating: 8}, {Rating: 7}, {Rating: 6}, {Rating: 5}},
			q:    1,
			want: 9,
		},
		{
			name: "q=0.5 returns the median (lower nearest-rank)",
			// descending: [9, 8, 7, 6, 5] → ascending index floor(0.5*4)=2 → 7
			eps:  []episode{{Rating: 9}, {Rating: 8}, {Rating: 7}, {Rating: 6}, {Rating: 5}},
			q:    0.5,
			want: 7,
		},
		{
			name: "q=0.9 over 9-episode fixture matches fixtureSeasons expectation",
			// ratings asc: [1,1,1,9,9,9,9.8,9.8,9.99] → index floor(0.9*8)=7 → 9.8
			eps: []episode{
				{Rating: 9.99}, {Rating: 9.8}, {Rating: 9}, {Rating: 9}, {Rating: 9},
				{Rating: 9}, {Rating: 1.1}, {Rating: 1}, {Rating: 1},
			},
			q:    0.9,
			want: 9.8,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := quantileRating(tc.eps, tc.q)
			require.InDelta(t, tc.want, got, 0.001)
		})
	}
}

func TestCountAboveStrict(t *testing.T) {
	testCases := []struct {
		name      string
		eps       []episode
		threshold float32
		want      int
	}{
		{name: "all above threshold", eps: []episode{{Rating: 9}, {Rating: 8}}, threshold: 7, want: 2},
		{name: "none above threshold", eps: []episode{{Rating: 5}, {Rating: 3}}, threshold: 7, want: 0},
		{name: "tied at threshold not counted", eps: []episode{{Rating: 9}, {Rating: 7}, {Rating: 5}}, threshold: 7, want: 1},
		{name: "empty slice returns 0", eps: nil, threshold: 5, want: 0},
		{name: "all equal to threshold returns 0", eps: []episode{{Rating: 5}, {Rating: 5}}, threshold: 5, want: 0},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, countAboveStrict(tc.eps, tc.threshold))
		})
	}
}

func TestIntersectSeasons(t *testing.T) {
	testCases := []struct {
		name      string
		requested []int
		available []int
		wantKeys  []int
	}{
		{
			name:      "empty requested returns empty keep (no filter)",
			requested: nil,
			available: []int{1, 2, 3},
			wantKeys:  nil,
		},
		{
			name:      "full overlap returns all requested",
			requested: []int{1, 2, 3},
			available: []int{1, 2, 3},
			wantKeys:  []int{1, 2, 3},
		},
		{
			name:      "partial overlap returns only matching",
			requested: []int{1, 4, 5},
			available: []int{1, 2, 3},
			wantKeys:  []int{1},
		},
		{
			name:      "no overlap returns empty keep",
			requested: []int{7, 8, 9},
			available: []int{1, 2, 3},
			wantKeys:  nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := intersectSeasons(tc.requested, tc.available)
			if len(tc.wantKeys) == 0 {
				require.Empty(t, got)
			} else {
				require.Len(t, got, len(tc.wantKeys))
				for _, k := range tc.wantKeys {
					_, ok := got[k]
					require.True(t, ok, "expected key %d in keep-set", k)
				}
			}
		})
	}
}

func TestAvailableSeasonNumbers(t *testing.T) {
	testCases := []struct {
		name    string
		seasons []*tvmeta.TVShowSeasonEpisodes
		want    []int
	}{
		{
			name:    "nil input returns empty",
			seasons: nil,
			want:    []int{},
		},
		{
			name: "seasons with zero episodes are excluded",
			seasons: []*tvmeta.TVShowSeasonEpisodes{
				{SeasonNumber: 1, Episodes: []*tvmeta.TVShowEpisode{}},
				{SeasonNumber: 2, Episodes: []*tvmeta.TVShowEpisode{{Number: 1}}},
				{SeasonNumber: 3, Episodes: nil},
			},
			want: []int{2},
		},
		{
			name: "nil season entry is skipped",
			seasons: []*tvmeta.TVShowSeasonEpisodes{
				nil,
				{SeasonNumber: 2, Episodes: []*tvmeta.TVShowEpisode{{Number: 1}}},
			},
			want: []int{2},
		},
		{
			name: "result is sorted ascending",
			seasons: []*tvmeta.TVShowSeasonEpisodes{
				{SeasonNumber: 3, Episodes: []*tvmeta.TVShowEpisode{{Number: 1}}},
				{SeasonNumber: 1, Episodes: []*tvmeta.TVShowEpisode{{Number: 1}}},
				{SeasonNumber: 2, Episodes: []*tvmeta.TVShowEpisode{{Number: 1}}},
			},
			want: []int{1, 2, 3},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := availableSeasonNumbers(tc.seasons)
			require.Equal(t, tc.want, got)
		})
	}
}
