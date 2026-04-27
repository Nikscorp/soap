package tvmeta

import (
	"context"
	"errors"
	"testing"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/stretchr/testify/require"
)

func popularResults(items ...popularItem) *tmdb.TVPopular {
	results := make([]struct {
		OriginalName     string   `json:"original_name"`
		GenreIDs         []int64  `json:"genre_ids"`
		Name             string   `json:"name"`
		Popularity       float32  `json:"popularity"`
		OriginCountry    []string `json:"origin_country"`
		FirstAirDate     string   `json:"first_air_date"`
		BackdropPath     string   `json:"backdrop_path"`
		OriginalLanguage string   `json:"original_language"`
		ID               int64    `json:"id"`
		Overview         string   `json:"overview"`
		PosterPath       string   `json:"poster_path"`
		tmdb.VoteMetrics
	}, 0, len(items))
	for _, it := range items {
		results = append(results, struct {
			OriginalName     string   `json:"original_name"`
			GenreIDs         []int64  `json:"genre_ids"`
			Name             string   `json:"name"`
			Popularity       float32  `json:"popularity"`
			OriginCountry    []string `json:"origin_country"`
			FirstAirDate     string   `json:"first_air_date"`
			BackdropPath     string   `json:"backdrop_path"`
			OriginalLanguage string   `json:"original_language"`
			ID               int64    `json:"id"`
			Overview         string   `json:"overview"`
			PosterPath       string   `json:"poster_path"`
			tmdb.VoteMetrics
		}{
			Name:         it.name,
			Popularity:   it.popularity,
			FirstAirDate: it.firstAirDate,
			ID:           it.id,
			Overview:     it.overview,
			PosterPath:   it.posterPath,
			VoteMetrics:  tmdb.VoteMetrics{VoteAverage: it.voteAverage},
		})
	}
	return &tmdb.TVPopular{
		TVAiringToday: &tmdb.TVAiringToday{
			TVAiringTodayResults: &tmdb.TVAiringTodayResults{
				Results: results,
			},
		},
	}
}

type popularItem struct {
	id           int64
	name         string
	popularity   float32
	firstAirDate string
	overview     string
	posterPath   string
	voteAverage  float32
}

func TestPopularTVShows(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetTVPopularMock.Set(func(urlOptions map[string]string) (*tmdb.TVPopular, error) {
		require.Equal(t, "en", urlOptions["language"])
		require.Equal(t, "1", urlOptions["page"])
		return popularResults(
			popularItem{id: 1, name: "Less Popular", popularity: 1, firstAirDate: "2020", posterPath: "/a.jpg", voteAverage: 7.0, overview: "a"},
			popularItem{id: 2, name: "More Popular", popularity: 999, firstAirDate: "2021", posterPath: "/b.jpg", voteAverage: 8.0, overview: "b"},
		), nil
	})

	resp, err := client.client.PopularTVShows(context.Background(), "en")
	require.NoError(t, err)
	require.Equal(t, []*TVShow{
		{ID: 2, Name: "More Popular", Rating: 8.0, Description: "b", PosterLink: "/img/b.jpg", FirstAirDate: "2021", Popularity: 999},
		{ID: 1, Name: "Less Popular", Rating: 7.0, Description: "a", PosterLink: "/img/a.jpg", FirstAirDate: "2020", Popularity: 1},
	}, resp)
}

func TestPopularTVShowsDefaultsLanguage(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetTVPopularMock.Set(func(urlOptions map[string]string) (*tmdb.TVPopular, error) {
		require.Equal(t, defaultLangTag, urlOptions["language"])
		return popularResults(), nil
	})

	resp, err := client.client.PopularTVShows(context.Background(), "")
	require.NoError(t, err)
	require.Empty(t, resp)
}

func TestPopularTVShowsError(t *testing.T) {
	client := NewClientM(t)
	someError := errors.New("boom")

	client.mockedTMDB.GetTVPopularMock.Set(func(_ map[string]string) (*tmdb.TVPopular, error) {
		return nil, someError
	})

	resp, err := client.client.PopularTVShows(context.Background(), "en")
	require.ErrorIs(t, err, someError)
	require.Nil(t, resp)
}

func TestPopularTVShowsNilResp(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetTVPopularMock.Set(func(_ map[string]string) (*tmdb.TVPopular, error) {
		return nil, nil
	})

	resp, err := client.client.PopularTVShows(context.Background(), "en")
	require.ErrorIs(t, err, ErrNilResp)
	require.Nil(t, resp)
}

func TestPopularTVShowsNilInner(t *testing.T) {
	client := NewClientM(t)

	client.mockedTMDB.GetTVPopularMock.Set(func(_ map[string]string) (*tmdb.TVPopular, error) {
		return &tmdb.TVPopular{}, nil
	})

	resp, err := client.client.PopularTVShows(context.Background(), "en")
	require.ErrorIs(t, err, ErrNilResp)
	require.Nil(t, resp)
}
