package lazysoap

import (
	"context"
	"errors"
	"fmt"
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
			Language: "en",
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
		{
			"searchResults": [
				{
					"id": 4815162342,
					"title": "Lost",
					"firstAirDate": "2022",
					"poster": "https://image.tmdb.org/t/p/w92/lost.jpg",
					"rating": 9.9
				}
			],
			"language": "en"
		}
	`, string(body))
	require.Equal(t, 1, len(srv.tvMetaClientMock.SearchTVShowsMock.Calls()))
}

func TestSearchHandlerUnicode(t *testing.T) {
	srv := NewServerM(t)
	defer srv.server.Close()

	srv.tvMetaClientMock.SearchTVShowsMock.Set(func(ctx context.Context, query string) (tp1 *tvmeta.TVShows, err error) {
		require.Equal(t, "Лост", query)
		return &tvmeta.TVShows{
			Language: "ru",
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
	fmt.Println(string(body))

	require.JSONEq(t, `
		{
			"searchResults": [
				{
					"id": 4815162342,
					"title": "Лост",
					"firstAirDate": "2022",
					"poster": "https://image.tmdb.org/t/p/w92/lost.jpg",
					"rating": 9.9
				}
			],
			"language": "ru"
		}
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
