package tvmeta

import (
	"context"
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

func TestCommon(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetSearchTVShowMock.Set(func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
		require.Equal(t, "Lost", query)
		require.Equal(t, 1, len(urlOptions))
		require.Equal(t, enLangTag, urlOptions["language"])

		return &tmdb.SearchTVShows{
			SearchTVShowsResults: &tmdb.SearchTVShowsResults{
				// ugly
				Results: []struct {
					OriginalName     string   `json:"original_name"`
					ID               int64    `json:"id"`
					Name             string   `json:"name"`
					VoteCount        int64    `json:"vote_count"`
					VoteAverage      float32  `json:"vote_average"`
					PosterPath       string   `json:"poster_path"`
					FirstAirDate     string   `json:"first_air_date"`
					Popularity       float32  `json:"popularity"`
					GenreIDs         []int64  `json:"genre_ids"`
					OriginalLanguage string   `json:"original_language"`
					BackdropPath     string   `json:"backdrop_path"`
					Overview         string   `json:"overview"`
					OriginCountry    []string `json:"origin_country"`
				}{
					{
						ID:           4815162342,
						Name:         "Lost",
						VoteAverage:  9.9,
						PosterPath:   "/lost.jpg",
						FirstAirDate: "2022",
						Overview:     "Greatest tv show ever",
					},
				},
			},
		}, nil
	})

	resp, err := client.client.SearchTVShows(context.Background(), "Lost")
	require.Equal(t, 1, len(client.mockedTMDB.GetSearchTVShowMock.Calls()))
	require.NoError(t, err)

	require.Equal(t, &TVShows{
		TVShows: []*TVShow{
			{
				ID:           4815162342,
				Name:         "Lost",
				Rating:       9.9,
				PosterLink:   "https://image.tmdb.org/t/p/w92/lost.jpg",
				FirstAirDate: "2022",
				Description:  "Greatest tv show ever",
			},
		},
	}, resp)
}
