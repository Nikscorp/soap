package lazysoap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nikscorp/soap/internal/app/lazysoap/mocks"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/stretchr/testify/require"
)

type ServerM struct {
	server           *httptest.Server
	tvMetaClientMock *mocks.TvMetaClientMock
}

func NewServerM(t *testing.T) *ServerM {
	tvMetaClientMock := mocks.NewTvMetaClientMock(t)
	srv := New("", tvMetaClientMock)
	ts := httptest.NewServer(srv.newRouter())

	return &ServerM{
		server:           ts,
		tvMetaClientMock: tvMetaClientMock,
	}
}

func TestSearchHandler(t *testing.T) {
	srv := NewServerM(t)
	defer srv.server.Close()

	srv.tvMetaClientMock.SearchTVShowsMock.Set(func(ctx context.Context, query string) (tp1 *tvmeta.TVShows, err error) {
		require.Equal(t, "Lost", query)
		return &tvmeta.TVShows{
			TVShows: []*tvmeta.TVShow{
				{
					ID:           4815162342,
					Name:         "Lost",
					Rating:       9.9,
					PosterLink:   "https://image.tmdb.org/t/p/w92/lost.jpg",
					FirstAirDate: "2022",
					Description:  "Greatest tv show ever",
				},
			},
		}, nil
	})

	resp, err := http.Get(srv.server.URL + "/search/Lost")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.JSONEq(t, `
		[
			{
				"title": "Lost",
				"imdbID": "4815162342",
				"year": "2022",
				"poster": "https://image.tmdb.org/t/p/w92/lost.jpg",
				"imdbRating": "9.9"
			}
		]
	`, string(body))
	require.Equal(t, 1, len(srv.tvMetaClientMock.SearchTVShowsMock.Calls()))
}

func TestSearchHandlerUnicode(t *testing.T) {
	srv := NewServerM(t)
	defer srv.server.Close()

	srv.tvMetaClientMock.SearchTVShowsMock.Set(func(ctx context.Context, query string) (tp1 *tvmeta.TVShows, err error) {
		require.Equal(t, "Лост", query)
		return &tvmeta.TVShows{
			TVShows: []*tvmeta.TVShow{
				{
					ID:           4815162342,
					Name:         "Лост",
					Rating:       9.9,
					PosterLink:   "https://image.tmdb.org/t/p/w92/lost.jpg",
					FirstAirDate: "2022",
					Description:  "Greatest tv show ever",
				},
			},
		}, nil
	})

	resp, err := http.Get(srv.server.URL + "/search/Лост")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.JSONEq(t, `
		[
			{
				"title": "Лост",
				"imdbID": "4815162342",
				"year": "2022",
				"poster": "https://image.tmdb.org/t/p/w92/lost.jpg",
				"imdbRating": "9.9"
			}
		]
	`, string(body))
	require.Equal(t, 1, len(srv.tvMetaClientMock.SearchTVShowsMock.Calls()))
}

func TestSearchHandlerTVMetaError(t *testing.T) {
	srv := NewServerM(t)
	defer srv.server.Close()

	srv.tvMetaClientMock.SearchTVShowsMock.Set(func(ctx context.Context, query string) (tp1 *tvmeta.TVShows, err error) {
		require.Equal(t, "Lost", query)
		return nil, errors.New("some error")
	})

	resp, err := http.Get(srv.server.URL + "/search/Lost")

	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "", string(body))
	require.Equal(t, 1, len(srv.tvMetaClientMock.SearchTVShowsMock.Calls()))
}
