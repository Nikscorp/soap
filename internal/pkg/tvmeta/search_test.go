package tvmeta

import (
	"context"
	"errors"
	"testing"

	"github.com/Nikscorp/soap/internal/pkg/tvmeta/mocks"
	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/stretchr/testify/require"
)

type ClientM struct {
	client     *Client
	mockedTMDB *mocks.TmdbClientMock
}

func NewClientM(t *testing.T) *ClientM {
	tmdbClient := mocks.NewTmdbClientMock(t)
	client := New(tmdbClient)
	return &ClientM{
		client:     client,
		mockedTMDB: tmdbClient,
	}
}

func TestSearchTVShows(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
		require.Equal(t, "Lost", query)
		require.Equal(t, 1, len(urlOptions))
		require.Equal(t, enLangTag, urlOptions["language"])

		return &tmdb.SearchTVShows{
			SearchTVShowsResults: &tmdb.SearchTVShowsResults{
				Results: []tmdb.TVShowResult{
					{
						ID:           4815162343,
						Name:         "Lost2",
						VoteMetrics:  tmdb.VoteMetrics{VoteAverage: 9.9},
						PosterPath:   "/lost.jpg",
						FirstAirDate: "2022",
						Overview:     "Greatest tv show ever",
						Popularity:   1,
					},
					{
						ID:           4815162342,
						Name:         "Lost",
						VoteMetrics:  tmdb.VoteMetrics{VoteAverage: 9.9},
						PosterPath:   "/lost.jpg",
						FirstAirDate: "2022",
						Overview:     "Greatest tv show ever",
						Popularity:   1000,
					},
				},
			},
		}, nil
	})

	resp, err := client.client.SearchTVShows(context.Background(), "Lost")
	require.Equal(t, 1, len(client.mockedTMDB.GetSearchTVShowMock.Calls()))
	require.NoError(t, err)

	// sorted by popularity desc
	require.Equal(t, &TVShows{
		Language: enLangTag,
		TVShows: []*TVShow{
			{
				ID:           4815162342,
				Name:         "Lost",
				Rating:       9.9,
				PosterLink:   "/img/lost.jpg",
				FirstAirDate: "2022",
				Description:  "Greatest tv show ever",
				Popularity:   1000,
			},
			{
				ID:           4815162343,
				Name:         "Lost2",
				Rating:       9.9,
				PosterLink:   "/img/lost.jpg",
				FirstAirDate: "2022",
				Description:  "Greatest tv show ever",
				Popularity:   1,
			},
		},
	}, resp)
}

func TestSearchTVShowsUnicode(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
		require.Equal(t, "Лост", query)
		require.Equal(t, 1, len(urlOptions))
		require.Equal(t, ruLangTag, urlOptions["language"])

		return &tmdb.SearchTVShows{
			SearchTVShowsResults: &tmdb.SearchTVShowsResults{
				Results: []tmdb.TVShowResult{
					{
						ID:           4815162342,
						Name:         "Лост",
						VoteMetrics:  tmdb.VoteMetrics{VoteAverage: 9.9},
						PosterPath:   "/lost.jpg",
						FirstAirDate: "2022",
						Overview:     "Greatest tv show ever",
						Popularity:   1000,
					},
				},
			},
		}, nil
	})

	resp, err := client.client.SearchTVShows(context.Background(), "Лост")
	require.Equal(t, 1, len(client.mockedTMDB.GetSearchTVShowMock.Calls()))
	require.NoError(t, err)

	// sorted by popularity desc
	require.Equal(t, &TVShows{
		Language: ruLangTag,
		TVShows: []*TVShow{
			{
				ID:           4815162342,
				Name:         "Лост",
				Rating:       9.9,
				PosterLink:   "/img/lost.jpg",
				FirstAirDate: "2022",
				Description:  "Greatest tv show ever",
				Popularity:   1000,
			},
		},
	}, resp)
}

func TestSearchTVShowsError(t *testing.T) {
	client := NewClientM(t)
	someError := errors.New("some error")

	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
		require.Equal(t, "Lost", query)
		require.Equal(t, 1, len(urlOptions))
		require.Equal(t, enLangTag, urlOptions["language"])

		return nil, someError
	})

	resp, err := client.client.SearchTVShows(context.Background(), "Lost")

	require.Equal(t, 1, len(client.mockedTMDB.GetSearchTVShowMock.Calls()))
	require.ErrorIs(t, err, someError)
	require.Equal(t, (*TVShows)(nil), resp)
}

func TestSearchTVShowsNilResp(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
		require.Equal(t, "Lost", query)
		require.Equal(t, 1, len(urlOptions))
		require.Equal(t, enLangTag, urlOptions["language"])

		return nil, nil
	})

	resp, err := client.client.SearchTVShows(context.Background(), "Lost")

	require.Equal(t, 1, len(client.mockedTMDB.GetSearchTVShowMock.Calls()))
	require.ErrorIs(t, err, ErrNilResp)
	require.Equal(t, (*TVShows)(nil), resp)
}

func TestSearchTVShowsNilTVShows(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
		require.Equal(t, "Lost", query)
		require.Equal(t, 1, len(urlOptions))
		require.Equal(t, enLangTag, urlOptions["language"])

		return &tmdb.SearchTVShows{
			SearchTVShowsResults: nil,
		}, nil
	})

	resp, err := client.client.SearchTVShows(context.Background(), "Lost")

	require.Equal(t, 1, len(client.mockedTMDB.GetSearchTVShowMock.Calls()))
	require.ErrorIs(t, err, ErrNilResp)
	require.Equal(t, (*TVShows)(nil), resp)
}

func TestSearchTVShowsNilResults(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
		require.Equal(t, "Lost", query)
		require.Equal(t, 1, len(urlOptions))
		require.Equal(t, enLangTag, urlOptions["language"])

		return &tmdb.SearchTVShows{
			SearchTVShowsResults: &tmdb.SearchTVShowsResults{
				Results: nil,
			},
		}, nil
	})

	resp, err := client.client.SearchTVShows(context.Background(), "Lost")

	require.Equal(t, 1, len(client.mockedTMDB.GetSearchTVShowMock.Calls()))
	require.ErrorIs(t, err, ErrNilResp)
	require.Equal(t, (*TVShows)(nil), resp)
}
