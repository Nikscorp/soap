package tvmeta

import (
	"context"
	"errors"
	"testing"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/stretchr/testify/require"
)

func TestTvShowDetails(t *testing.T) {
	client := NewClientM(t)
	client.mockedTMDB.GetTVDetailsMock.Set(func(id int, urlOptions map[string]string) (tp1 *tmdb.TVDetails, err error) {
		require.Equal(t, 42, id)
		return &tmdb.TVDetails{
			Name:            "Lost",
			NumberOfSeasons: 23,
			PosterPath:      "/lost.png",
		}, nil
	})

	resp, err := client.client.TvShowDetails(context.Background(), 42)

	require.Equal(t, 1, len(client.mockedTMDB.GetTVDetailsMock.Calls()))
	require.NoError(t, err)
	require.Equal(t, &TvShowDetails{
		ID:         42,
		Title:      "Lost",
		PosterLink: "/img/lost.png",
		SeasonsCnt: 23,
	}, resp)
}

func TestTvShowDetailsErrorResp(t *testing.T) {
	client := NewClientM(t)
	someError := errors.New("some error")
	client.mockedTMDB.GetTVDetailsMock.Set(func(id int, urlOptions map[string]string) (tp1 *tmdb.TVDetails, err error) {
		require.Equal(t, 42, id)
		return nil, someError
	})

	resp, err := client.client.TvShowDetails(context.Background(), 42)

	require.Equal(t, 1, len(client.mockedTMDB.GetTVDetailsMock.Calls()))
	require.ErrorIs(t, err, someError)
	require.Equal(t, (*TvShowDetails)(nil), resp)
}

func TestTvShowDetailsNilResp(t *testing.T) {
	client := NewClientM(t)
	client.mockedTMDB.GetTVDetailsMock.Set(func(id int, urlOptions map[string]string) (tp1 *tmdb.TVDetails, err error) {
		require.Equal(t, 42, id)
		return nil, nil
	})

	resp, err := client.client.TvShowDetails(context.Background(), 42)

	require.Equal(t, 1, len(client.mockedTMDB.GetTVDetailsMock.Calls()))
	require.ErrorIs(t, err, ErrNilResp)
	require.Equal(t, (*TvShowDetails)(nil), resp)
}
