package tvmeta

import (
	"context"
	"errors"
	"testing"
	"time"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/stretchr/testify/require"
)

// episodesCacheCfg returns a CacheConfig with the episodes cache enabled at a
// generous size and TTL — large enough that no test-grade timing flake can
// expire entries mid-test.
func episodesCacheCfg() CacheConfig {
	return CacheConfig{EpisodesSize: 64, EpisodesTTL: time.Minute}
}

type tmdbEpisode = struct {
	AirDate        string `json:"air_date"`
	EpisodeNumber  int    `json:"episode_number"`
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Overview       string `json:"overview"`
	ProductionCode string `json:"production_code"`
	Runtime        int    `json:"runtime"`
	SeasonNumber   int    `json:"season_number"`
	ShowID         int64  `json:"show_id"`
	StillPath      string `json:"still_path"`
	tmdb.VoteMetrics
	Crew []struct {
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
							VoteMetrics:   tmdb.VoteMetrics{VoteAverage: 9.99},
							StillPath:     "/abc.jpg",
						},
						{
							EpisodeNumber: 2,
							Name:          "Second One",
							Overview:      "No still here",
							VoteMetrics:   tmdb.VoteMetrics{VoteAverage: 8.0},
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
						StillLink:   "/img/abc.jpg",
					},
					{
						Number:      2,
						Name:        "Second One",
						Description: "No still here",
						Rating:      8.0,
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

// TestTVShowEpisodesBySeasonCacheHitsOnce verifies the per-(id, season, lang)
// episodes cache: repeated calls with the same key issue exactly one TMDB
// season-details request.
func TestTVShowEpisodesBySeasonCacheHitsOnce(t *testing.T) {
	client := NewClientMCfg(t, episodesCacheCfg())
	client.mockedTMDB.GetTVSeasonDetailsMock.Return(&tmdb.TVSeasonDetails{
		Episodes: []tmdbEpisode{
			{EpisodeNumber: 1, Name: "Ep1", VoteMetrics: tmdb.VoteMetrics{VoteAverage: 7.5}},
		},
	}, nil)

	for range 5 {
		resp, err := client.client.TVShowEpisodesBySeason(context.Background(), 42, 1, "en")
		require.NoError(t, err)
		require.Len(t, resp.Episodes, 1)
	}

	require.Equal(t, uint64(1), client.mockedTMDB.GetTVSeasonDetailsAfterCounter(),
		"cache must collapse repeated lookups to a single TMDB call")
}

// TestTVShowEpisodesBySeasonCacheKeyIsolation verifies that distinct
// (id, season, lang) tuples do not collide: each unique key fetches
// independently.
func TestTVShowEpisodesBySeasonCacheKeyIsolation(t *testing.T) {
	client := NewClientMCfg(t, episodesCacheCfg())
	client.mockedTMDB.GetTVSeasonDetailsMock.Set(func(id, seasonNumber int, urlOptions map[string]string) (*tmdb.TVSeasonDetails, error) {
		// Encode the tuple in episode name so we can assert no collisions later.
		return &tmdb.TVSeasonDetails{
			Episodes: []tmdbEpisode{
				{EpisodeNumber: 1, Name: urlOptions[langOptKey], VoteMetrics: tmdb.VoteMetrics{VoteAverage: float32(id*100 + seasonNumber)}},
			},
		}, nil
	})

	r1, err := client.client.TVShowEpisodesBySeason(context.Background(), 42, 1, "en")
	require.NoError(t, err)
	require.Equal(t, "en", r1.Episodes[0].Name)
	require.InDelta(t, 4201, r1.Episodes[0].Rating, 0.001)

	// Different season — must not collide.
	r2, err := client.client.TVShowEpisodesBySeason(context.Background(), 42, 2, "en")
	require.NoError(t, err)
	require.InDelta(t, 4202, r2.Episodes[0].Rating, 0.001)

	// Different id — must not collide.
	r3, err := client.client.TVShowEpisodesBySeason(context.Background(), 43, 1, "en")
	require.NoError(t, err)
	require.InDelta(t, 4301, r3.Episodes[0].Rating, 0.001)

	// Different lang — must not collide.
	r4, err := client.client.TVShowEpisodesBySeason(context.Background(), 42, 1, "ru")
	require.NoError(t, err)
	require.Equal(t, "ru", r4.Episodes[0].Name)

	// Subsequent identical lookups must hit the cache.
	for range 3 {
		_, err = client.client.TVShowEpisodesBySeason(context.Background(), 42, 1, "en")
		require.NoError(t, err)
		_, err = client.client.TVShowEpisodesBySeason(context.Background(), 42, 2, "en")
		require.NoError(t, err)
		_, err = client.client.TVShowEpisodesBySeason(context.Background(), 43, 1, "en")
		require.NoError(t, err)
		_, err = client.client.TVShowEpisodesBySeason(context.Background(), 42, 1, "ru")
		require.NoError(t, err)
	}

	require.Equal(t, uint64(4), client.mockedTMDB.GetTVSeasonDetailsAfterCounter(),
		"distinct (id, season, lang) keys must each fetch exactly once")
}

// TestTVShowEpisodesBySeasonCacheErrorNotCached verifies that a TMDB error is
// returned to the caller and NOT cached: the next call re-issues the
// underlying request.
func TestTVShowEpisodesBySeasonCacheErrorNotCached(t *testing.T) {
	client := NewClientMCfg(t, episodesCacheCfg())
	someError := errors.New("some error")
	client.mockedTMDB.GetTVSeasonDetailsMock.Return(nil, someError)

	for range 2 {
		resp, err := client.client.TVShowEpisodesBySeason(context.Background(), 42, 1, "en")
		require.ErrorIs(t, err, someError)
		require.Nil(t, resp)
	}

	require.Equal(t, uint64(2), client.mockedTMDB.GetTVSeasonDetailsAfterCounter(),
		"errors must not be cached; both calls must hit TMDB")
}
