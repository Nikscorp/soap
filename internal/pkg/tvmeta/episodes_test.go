package tvmeta

import (
	"context"
	"errors"
	"testing"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/stretchr/testify/require"
)

func TestTVShowEpisodesBySeason(t *testing.T) {
	client := NewClientM(t)
	client.mockedTMDB.GetTVSeasonDetailsMock.Set(func(id, seasonNumber int, urlOptions map[string]string) (tp1 *tmdb.TVSeasonDetails, err error) {
		return &tmdb.TVSeasonDetails{
			Episodes: []struct {
				AirDate        string  `json:"air_date"`
				EpisodeNumber  int     `json:"episode_number"`
				ID             int64   `json:"id"`
				Name           string  `json:"name"`
				Overview       string  `json:"overview"`
				ProductionCode string  `json:"production_code"`
				SeasonNumber   int     `json:"season_number"`
				ShowID         int64   `json:"show_id"`
				StillPath      string  `json:"still_path"`
				VoteAverage    float32 `json:"vote_average"`
				VoteCount      int64   `json:"vote_count"`
				Crew           []struct {
					ID          int64  `json:"id"`
					CreditID    string `json:"credit_id"`
					Name        string `json:"name"`
					Department  string `json:"department"`
					Job         string `json:"job"`
					Gender      int    `json:"gender"`
					ProfilePath string `json:"profile_path"`
				} `json:"crew"`
				GuestStars []struct {
					ID          int64  `json:"id"`
					Name        string `json:"name"`
					CreditID    string `json:"credit_id"`
					Character   string `json:"character"`
					Order       int    `json:"order"`
					Gender      int    `json:"gender"`
					ProfilePath string `json:"profile_path"`
				} `json:"guest_stars"`
			}{
				{
					EpisodeNumber: 1,
					Name:          "First One",
					Overview:      "Greatest episode ever",
					VoteAverage:   9.99,
				},
			},
		}, nil
	})

	resp, err := client.client.TVShowEpisodesBySeason(context.Background(), 42, 1)
	require.NoError(t, err)
	require.Equal(t, 1, len(client.mockedTMDB.GetTVSeasonDetailsMock.Calls()))
	require.Equal(t, &TVShowSeasonEpisodes{
		SeasonNumber: 1,
		Episodes: []*TVShowEpisode{
			{
				Number:      1,
				Name:        "First One",
				Description: "Greatest episode ever",
				Rating:      9.99,
			},
		},
	}, resp)
}

func TestTVShowEpisodesBySeasonError(t *testing.T) {
	client := NewClientM(t)
	someError := errors.New("some error")
	client.mockedTMDB.GetTVSeasonDetailsMock.Set(func(id, seasonNumber int, urlOptions map[string]string) (tp1 *tmdb.TVSeasonDetails, err error) {
		return nil, someError
	})

	resp, err := client.client.TVShowEpisodesBySeason(context.Background(), 42, 1)
	require.ErrorIs(t, err, someError)
	require.Equal(t, 1, len(client.mockedTMDB.GetTVSeasonDetailsMock.Calls()))
	require.Equal(t, (*TVShowSeasonEpisodes)(nil), resp)
}

func TestTVShowEpisodesBySeasonNilResp(t *testing.T) {
	client := NewClientM(t)
	client.mockedTMDB.GetTVSeasonDetailsMock.Set(func(id, seasonNumber int, urlOptions map[string]string) (tp1 *tmdb.TVSeasonDetails, err error) {
		return nil, nil
	})

	resp, err := client.client.TVShowEpisodesBySeason(context.Background(), 42, 1)
	require.ErrorIs(t, err, ErrNilResp)
	require.Equal(t, 1, len(client.mockedTMDB.GetTVSeasonDetailsMock.Calls()))
	require.Equal(t, (*TVShowSeasonEpisodes)(nil), resp)
}
