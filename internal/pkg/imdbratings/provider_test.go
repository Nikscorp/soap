package imdbratings

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleRatings = "tconst\taverageRating\tnumVotes\n" +
	"tt0944947\t9.2\t2300000\n" + // Game of Thrones series
	"tt1480055\t9.1\t52000\n" + //   GoT s01e01
	"tt1480056\t8.8\t40000\n" + //   GoT s01e02
	"tt1480057\t\\N\t\\N\n" + //     null row, must be skipped
	"tt0306414\t9.3\t380000\n" + //  The Wire series
	"tt0306414bad\tnotanumber\t10\n" // bad rating row, must be skipped

const sampleEpisodes = "tconst\tparentTconst\tseasonNumber\tepisodeNumber\n" +
	"tt1480055\ttt0944947\t1\t1\n" +
	"tt1480056\ttt0944947\t1\t2\n" +
	"tt1480058\ttt0944947\t\\N\t\\N\n" + // unknown season/ep, skipped
	"tt9999999\ttt0944947\t1\t3\n" + //    no rating row in titles, skipped
	"tt0306414bad\ttt0306414\t1\t1\n" //   parent has rating but row was bad-skipped earlier, still has parent rating in titles? actually no — tt0306414bad was skipped from titles too

func gzipBytes(t testing.TB, raw string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write([]byte(raw))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestParseRatings(t *testing.T) {
	titles, err := parseRatings(bytes.NewReader(gzipBytes(t, sampleRatings)))
	require.NoError(t, err)

	got, ok := titles["tt0944947"]
	require.True(t, ok)
	assert.InDelta(t, 9.2, got.Rating, 0.001)
	assert.Equal(t, uint32(2_300_000), got.Votes)

	_, ok = titles["tt1480057"]
	assert.False(t, ok, "rows with \\N must be skipped")

	_, ok = titles["tt0306414bad"]
	assert.False(t, ok, "rows with non-numeric rating must be skipped")

	assert.Len(t, titles, 4)
}

func TestParseEpisodesJoinAndSort(t *testing.T) {
	titles, err := parseRatings(bytes.NewReader(gzipBytes(t, sampleRatings)))
	require.NoError(t, err)

	episodes, err := parseEpisodes(bytes.NewReader(gzipBytes(t, sampleEpisodes)), titles)
	require.NoError(t, err)

	gotSeries, ok := episodes["tt0944947"]
	require.True(t, ok)
	require.Len(t, gotSeries, 2, "only the two episodes with ratings in titles should join")

	// Sorted by (season, episode) ascending.
	assert.Equal(t, int16(1), gotSeries[0].Season)
	assert.Equal(t, int16(1), gotSeries[0].Episode)
	assert.InDelta(t, 9.1, gotSeries[0].Rating, 0.001)
	assert.Equal(t, int16(1), gotSeries[1].Season)
	assert.Equal(t, int16(2), gotSeries[1].Episode)
}

func TestProviderLookups(t *testing.T) {
	titles, err := parseRatings(bytes.NewReader(gzipBytes(t, sampleRatings)))
	require.NoError(t, err)
	episodes, err := parseEpisodes(bytes.NewReader(gzipBytes(t, sampleEpisodes)), titles)
	require.NoError(t, err)

	p := &Provider{}
	assert.False(t, p.Ready(), "Ready must be false before any snapshot is stored")

	p.snap.Store(&snapshot{titles: titles, episodes: episodes})
	assert.True(t, p.Ready())

	t.Run("series rating hit", func(t *testing.T) {
		r, v, ok := p.SeriesRating("tt0944947")
		require.True(t, ok)
		assert.InDelta(t, 9.2, r, 0.001)
		assert.Equal(t, uint32(2_300_000), v)
	})

	t.Run("series rating miss", func(t *testing.T) {
		_, _, ok := p.SeriesRating("tt0000000")
		assert.False(t, ok)
	})

	t.Run("series rating empty input", func(t *testing.T) {
		_, _, ok := p.SeriesRating("")
		assert.False(t, ok)
	})

	t.Run("episode rating hit", func(t *testing.T) {
		r, _, ok := p.EpisodeRating("tt0944947", 1, 2)
		require.True(t, ok)
		assert.InDelta(t, 8.8, r, 0.001)
	})

	t.Run("episode rating miss season/ep", func(t *testing.T) {
		_, _, ok := p.EpisodeRating("tt0944947", 99, 1)
		assert.False(t, ok)
	})

	t.Run("episode rating unknown series", func(t *testing.T) {
		_, _, ok := p.EpisodeRating("tt0000000", 1, 1)
		assert.False(t, ok)
	})

	t.Run("episode rating no snapshot", func(t *testing.T) {
		empty := &Provider{}
		_, _, ok := empty.EpisodeRating("tt0944947", 1, 1)
		assert.False(t, ok)
	})
}

// TestRunFetchesAndPublishes spins up an httptest server returning two
// gzipped TSVs and verifies that Run loads them into a published snapshot.
// Uses a tight context so the function returns after the initial refresh.
func TestRunFetchesAndPublishes(t *testing.T) {
	ratingsGz := gzipBytes(t, sampleRatings)
	episodeGz := gzipBytes(t, sampleEpisodes)

	mux := http.NewServeMux()
	mux.HandleFunc("/"+ratingsFile, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(ratingsGz)
	})
	mux.HandleFunc("/"+episodeFile, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(episodeGz)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tmp := t.TempDir()
	p := New(Config{
		DatasetsBaseURL: srv.URL,
		RefreshInterval: 0, // run once and exit
		CacheDir:        tmp,
		HTTPTimeout:     5 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p.Run(ctx)

	require.True(t, p.Ready(), "snapshot must be published after Run")

	r, _, ok := p.SeriesRating("tt0944947")
	require.True(t, ok)
	assert.InDelta(t, 9.2, r, 0.001)

	// Cache files should exist on disk after a successful fetch.
	for _, name := range []string{ratingsFile, episodeFile} {
		st, err := os.Stat(filepath.Join(tmp, name))
		require.NoError(t, err)
		assert.Greater(t, st.Size(), int64(0))
	}
}

// TestRunPrunesNonSeriesTitles verifies the pruneTitlesToSeries pass: after
// a successful build, SeriesRating must miss for tconsts that are rated but
// not parents of any episode (movies, individual episodes, shorts), and hit
// for series tconsts that have rated episodes.
func TestRunPrunesNonSeriesTitles(t *testing.T) {
	ratingsGz := gzipBytes(t, sampleRatings)
	episodeGz := gzipBytes(t, sampleEpisodes)

	mux := http.NewServeMux()
	mux.HandleFunc("/"+ratingsFile, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(ratingsGz)
	})
	mux.HandleFunc("/"+episodeFile, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(episodeGz)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := New(Config{
		DatasetsBaseURL: srv.URL,
		RefreshInterval: 0,
		CacheDir:        t.TempDir(),
		HTTPTimeout:     5 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p.Run(ctx)

	require.True(t, p.Ready())

	// tt1480055 is a rated episode of GoT, NOT a series tconst — it must be
	// pruned out. (TMDB external_ids returns series tconsts, never episode
	// tconsts, so this lookup never happens in practice; the assertion is a
	// guard against accidentally leaking non-series ratings if the lookup
	// shape ever changes.)
	_, _, ok := p.SeriesRating("tt1480055")
	assert.False(t, ok, "episode tconst must be pruned out of titles")

	// Sanity: a real series with rated episodes still resolves.
	r, _, ok := p.SeriesRating("tt0944947")
	require.True(t, ok)
	assert.InDelta(t, 9.2, r, 0.001)

	// Episode-level lookup still works through the unrelated episodes map.
	er, _, ok := p.EpisodeRating("tt0944947", 1, 1)
	require.True(t, ok)
	assert.InDelta(t, 9.1, er, 0.001)
}

// TestRefreshSkipsDownloadWhenCacheIsFresh asserts that on a hot restart we
// don't re-download the ~62 MB dataset when the on-disk cache is younger
// than RefreshInterval. The HTTP server here would return a body that
// disagrees with the cache; if we ever silently re-download, the assertion
// on the cached rating value catches it.
func TestRefreshSkipsDownloadWhenCacheIsFresh(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ratingsFile), gzipBytes(t, sampleRatings), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, episodeFile), gzipBytes(t, sampleEpisodes), 0o644))

	// Server that returns DIFFERENT bytes on the off chance we hit it. If we
	// hit the network when we shouldn't, the rating for tt0944947 would
	// change to 1.0 and the assertion would catch it.
	tampered := "tconst\taverageRating\tnumVotes\ntt0944947\t1.0\t1\n"
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write(gzipBytes(t, tampered))
	}))
	defer srv.Close()

	p := New(Config{
		DatasetsBaseURL: srv.URL,
		RefreshInterval: time.Hour, // freshly-written files are well under 1h old
		CacheDir:        tmp,
		HTTPTimeout:     2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p.Run(ctx)

	require.True(t, p.Ready())
	assert.Equal(t, int32(0), hits.Load(), "fresh cache must not trigger any HTTP fetch")

	r, _, ok := p.SeriesRating("tt0944947")
	require.True(t, ok)
	assert.InDelta(t, 9.2, r, 0.001, "rating must come from the cached file, not the tampered server")
}

// TestRefreshFallsBackToStaleCacheOnDownloadFailure: the on-disk cache is
// older than RefreshInterval (so the freshness check requests a download),
// the download fails (server returns 500), and the provider keeps using the
// stale cache. The previous test name was the same scenario but masked by a
// RefreshInterval=0 short-circuit.
func TestRefreshFallsBackToStaleCacheOnDownloadFailure(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ratingsFile), gzipBytes(t, sampleRatings), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, episodeFile), gzipBytes(t, sampleEpisodes), 0o644))

	// Backdate both files so the freshness check considers them stale.
	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(filepath.Join(tmp, ratingsFile), old, old))
	require.NoError(t, os.Chtimes(filepath.Join(tmp, episodeFile), old, old))

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := New(Config{
		DatasetsBaseURL: srv.URL,
		RefreshInterval: 24 * time.Hour, // stale (48h > 24h)
		CacheDir:        tmp,
		HTTPTimeout:     2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p.Run(ctx)

	require.True(t, p.Ready(), "stale on-disk file must be enough to publish a snapshot when download fails")
	assert.GreaterOrEqual(t, hits.Load(), int32(1), "stale cache must trigger a download attempt")

	r, _, ok := p.SeriesRating("tt0944947")
	require.True(t, ok)
	assert.InDelta(t, 9.2, r, 0.001)
}

// TestCachedFileAge unit-tests the freshness helper across the four shapes
// that matter in production.
func TestCachedFileAge(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "nope")
	present := filepath.Join(tmp, "present")
	require.NoError(t, os.WriteFile(present, []byte("x"), 0o644))

	t.Run("missing file is never fresh", func(t *testing.T) {
		_, fresh := cachedFileAge(missing, time.Hour)
		assert.False(t, fresh)
	})
	t.Run("present + young + maxAge>0 → fresh", func(t *testing.T) {
		_, fresh := cachedFileAge(present, time.Hour)
		assert.True(t, fresh)
	})
	t.Run("present + maxAge=0 → always fresh", func(t *testing.T) {
		_, fresh := cachedFileAge(present, 0)
		assert.True(t, fresh, "maxAge=0 means refresh disabled — accept any cache")
	})
	t.Run("present + old + maxAge>0 → stale", func(t *testing.T) {
		old := time.Now().Add(-2 * time.Hour)
		require.NoError(t, os.Chtimes(present, old, old))
		_, fresh := cachedFileAge(present, time.Hour)
		assert.False(t, fresh)
	})
}

// TestParseRatingsHandlesTrailingNewlinesAndCRLF — defensive coverage for
// CRLF-terminated lines and stray blank lines. bufio.Scanner's ScanLines
// strips trailing \r, so well-formed rows still parse; blank lines must
// not panic or pollute the map.
func TestParseRatingsHandlesTrailingNewlinesAndCRLF(t *testing.T) {
	raw := "tconst\taverageRating\tnumVotes\r\n" +
		"tt1\t8.0\t100\r\n" +
		"\r\n" + // blank line
		"tt2\t7.0\t200\n"
	titles, err := parseRatings(bytes.NewReader(gzipBytes(t, raw)))
	require.NoError(t, err)

	got1, ok := titles["tt1"]
	require.True(t, ok)
	assert.InDelta(t, 8.0, got1.Rating, 0.001)
	assert.Equal(t, uint32(100), got1.Votes)

	got2, ok := titles["tt2"]
	require.True(t, ok)
	assert.InDelta(t, 7.0, got2.Rating, 0.001)

	// No empty-key entry from the blank line.
	_, ok = titles[""]
	assert.False(t, ok)

	// Sanity: input really did contain a blank CRLF line.
	assert.True(t, strings.Contains(raw, "\r\n\r\n"))
}
