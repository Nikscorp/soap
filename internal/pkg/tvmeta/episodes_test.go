package tvmeta

import (
	"context"
	"errors"
	"testing"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/stretchr/testify/require"
)

type tmdbEpisode = struct {
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
}

func TestTVShowEpisodesBySeason(t *testing.T) {
	testCases := []struct {
		name      string
		inputLang string
		wantLang  string
	}{
		{
			name:      "default as input",
			inputLang: defaultLangTag,
			wantLang:  defaultLangTag,
		},
		{
			name:      "ruLangTag",
			inputLang: ruLangTag,
			wantLang:  ruLangTag,
		},
		{
			name:      "empty lang",
			inputLang: "",
			wantLang:  defaultLangTag,
		},
		{
			name:      "enLangTag",
			inputLang: enLangTag,
			wantLang:  enLangTag,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClientM(t)
			client.mockedTMDB.GetTVSeasonDetailsMock.Set(func(id, seasonNumber int, urlOptions map[string]string) (tp1 *tmdb.TVSeasonDetails, err error) {
				require.Equal(t, map[string]string{"language": tc.wantLang}, urlOptions)
				return &tmdb.TVSeasonDetails{
					Episodes: []tmdbEpisode{
						{
							EpisodeNumber: 1,
							Name:          "First One",
							Overview:      "Greatest episode ever",
							VoteAverage:   9.99,
						},
					},
				}, nil
			})

			resp, err := client.client.TVShowEpisodesBySeason(context.Background(), 42, 1, tc.inputLang)
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
		})
	}
}

func TestTVShowEpisodesBySeasonError(t *testing.T) {
	client := NewClientM(t)
	someError := errors.New("some error")
	client.mockedTMDB.GetTVSeasonDetailsMock.Set(func(id, seasonNumber int, urlOptions map[string]string) (tp1 *tmdb.TVSeasonDetails, err error) {
		require.Equal(t, map[string]string{"language": defaultLangTag}, urlOptions)
		return nil, someError
	})

	resp, err := client.client.TVShowEpisodesBySeason(context.Background(), 42, 1, defaultLangTag)
	require.ErrorIs(t, err, someError)
	require.Equal(t, 1, len(client.mockedTMDB.GetTVSeasonDetailsMock.Calls()))
	require.Equal(t, (*TVShowSeasonEpisodes)(nil), resp)
}

func TestTVShowEpisodesBySeasonNilResp(t *testing.T) {
	client := NewClientM(t)
	client.mockedTMDB.GetTVSeasonDetailsMock.Set(func(id, seasonNumber int, urlOptions map[string]string) (tp1 *tmdb.TVSeasonDetails, err error) {
		require.Equal(t, map[string]string{"language": defaultLangTag}, urlOptions)
		return nil, nil
	})

	resp, err := client.client.TVShowEpisodesBySeason(context.Background(), 42, 1, defaultLangTag)
	require.ErrorIs(t, err, ErrNilResp)
	require.Equal(t, 1, len(client.mockedTMDB.GetTVSeasonDetailsMock.Calls()))
	require.Equal(t, (*TVShowSeasonEpisodes)(nil), resp)
}
