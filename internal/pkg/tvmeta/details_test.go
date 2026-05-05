package tvmeta

import (
	"context"
	"errors"
	"testing"
	"time"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/stretchr/testify/require"
)

// detailsCacheCfg returns a CacheConfig with the details cache enabled at a
// generous size and TTL — large enough that no test-grade timing flake can
// expire entries mid-test.
func detailsCacheCfg() CacheConfig {
	return CacheConfig{DetailsSize: 64, DetailsTTL: time.Minute}
}

func TestTvShowDetails(t *testing.T) {
	client := NewClientM(t)
	client.mockedTMDB.GetTVDetailsMock.Set(func(id int, urlOptions map[string]string) (tp1 *tmdb.TVDetails, err error) {
		require.Equal(t, 42, id)
		require.Equal(t, "de", urlOptions["language"])
		return &tmdb.TVDetails{
			Name:            "Lost",
			NumberOfSeasons: 23,
			PosterPath:      "/lost.png",
			FirstAirDate:    "2004-09-22",
			Overview:        "A plane crashes on a mysterious island.",
		}, nil
	})

	resp, err := client.client.TVShowDetails(context.Background(), 42, "de")

	require.Equal(t, 1, len(client.mockedTMDB.GetTVDetailsMock.Calls()))
	require.NoError(t, err)
	require.Equal(t, &TvShowDetails{
		ID:           42,
		Title:        "Lost",
		PosterLink:   "/img/lost.png",
		SeasonsCnt:   23,
		FirstAirDate: "2004-09-22",
		Overview:     "A plane crashes on a mysterious island.",
	}, resp)
}

func TestTvShowDetailsEmptyLanguageOmitsOption(t *testing.T) {
	client := NewClientM(t)
	client.mockedTMDB.GetTVDetailsMock.Set(func(id int, urlOptions map[string]string) (tp1 *tmdb.TVDetails, err error) {
		require.Equal(t, 42, id)
		require.Nil(t, urlOptions)
		return &tmdb.TVDetails{Name: "Lost"}, nil
	})

	_, err := client.client.TVShowDetails(context.Background(), 42, "")

	require.NoError(t, err)
}

func TestTvShowDetailsErrorResp(t *testing.T) {
	client := NewClientM(t)
	someError := errors.New("some error")
	client.mockedTMDB.GetTVDetailsMock.Set(func(id int, urlOptions map[string]string) (tp1 *tmdb.TVDetails, err error) {
		require.Equal(t, 42, id)
		return nil, someError
	})

	resp, err := client.client.TVShowDetails(context.Background(), 42, "")

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

	resp, err := client.client.TVShowDetails(context.Background(), 42, "")

	require.Equal(t, 1, len(client.mockedTMDB.GetTVDetailsMock.Calls()))
	require.ErrorIs(t, err, ErrNilResp)
	require.Equal(t, (*TvShowDetails)(nil), resp)
}

// TestTvShowDetailsCacheHitsOnce verifies the per-(id, lang) details cache:
// repeated calls with the same key issue exactly one TMDB request.
func TestTvShowDetailsCacheHitsOnce(t *testing.T) {
	client := NewClientMCfg(t, detailsCacheCfg())
	client.mockedTMDB.GetTVDetailsMock.Return(&tmdb.TVDetails{Name: "Lost"}, nil)

	for range 5 {
		resp, err := client.client.TVShowDetails(context.Background(), 42, "en")
		require.NoError(t, err)
		require.Equal(t, "Lost", resp.Title)
	}

	require.Equal(t, uint64(1), client.mockedTMDB.GetTVDetailsAfterCounter(),
		"cache must collapse repeated lookups to a single TMDB call")
}

// TestTvShowDetailsCacheLanguageIsolation verifies that different language
// keys do not collide: each (id, lang) pair fetches independently.
func TestTvShowDetailsCacheLanguageIsolation(t *testing.T) {
	client := NewClientMCfg(t, detailsCacheCfg())
	client.mockedTMDB.GetTVDetailsMock.Set(func(_ int, urlOptions map[string]string) (*tmdb.TVDetails, error) {
		return &tmdb.TVDetails{Name: urlOptions[langOptKey]}, nil
	})

	en, err := client.client.TVShowDetails(context.Background(), 42, "en")
	require.NoError(t, err)
	require.Equal(t, "en", en.Title)

	ru, err := client.client.TVShowDetails(context.Background(), 42, "ru")
	require.NoError(t, err)
	require.Equal(t, "ru", ru.Title)

	// Subsequent requests for each language must hit the cache.
	for range 3 {
		_, err = client.client.TVShowDetails(context.Background(), 42, "en")
		require.NoError(t, err)
		_, err = client.client.TVShowDetails(context.Background(), 42, "ru")
		require.NoError(t, err)
	}

	require.Equal(t, uint64(2), client.mockedTMDB.GetTVDetailsAfterCounter(),
		"distinct lang keys must each fetch exactly once")
}

// TestTvShowDetailsCacheErrorNotCached verifies that a TMDB error is returned
// to the caller and NOT cached: the next call re-issues the underlying request.
func TestTvShowDetailsCacheErrorNotCached(t *testing.T) {
	client := NewClientMCfg(t, detailsCacheCfg())
	someError := errors.New("some error")
	client.mockedTMDB.GetTVDetailsMock.Return(nil, someError)

	for range 2 {
		resp, err := client.client.TVShowDetails(context.Background(), 42, "en")
		require.ErrorIs(t, err, someError)
		require.Nil(t, resp)
	}

	require.Equal(t, uint64(2), client.mockedTMDB.GetTVDetailsAfterCounter(),
		"errors must not be cached; both calls must hit TMDB")
}
