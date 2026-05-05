package imdbratings

import (
	"bytes"
	"compress/gzip"
	"context"
	"math"
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

	got, ok := titles[mustParseTconst(t, "tt0944947")]
	require.True(t, ok)
	assert.InDelta(t, 9.2, got.Rating, 0.001)
	assert.Equal(t, uint32(2_300_000), got.Votes)

	_, ok = titles[mustParseTconst(t, "tt1480057")]
	assert.False(t, ok, "rows with \\N must be skipped")

	// "tt0306414bad" has a "bad" suffix so parseTconst rejects it before
	// we even reach the float parse — the row is skipped either way.
	_, parseable := parseTconst("tt0306414bad")
	assert.False(t, parseable, "tconst with non-digit suffix must not parse")

	assert.Len(t, titles, 4)
}

// mustParseTconst is a tiny test helper that parses a tconst string and
// fails the test if it doesn't. Used wherever tests want to look up an
// entry by its public string ID in the new uint32-keyed snapshot maps.
func mustParseTconst(t testing.TB, s string) uint32 {
	t.Helper()
	id, ok := parseTconst(s)
	require.True(t, ok, "parseTconst(%q) must succeed", s)
	return id
}

func TestParseTconst(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		cases := map[string]uint32{
			"tt0000001": 1,
			"tt0944947": 944947,
			"tt1":       1,                // minimum valid (tt + 1 digit)
			"tt9999999": 9_999_999,        // typical IMDb 7-digit tconst
			"tt40000000": 40_000_000,      // beyond 2026's largest, still fits uint32
			"tt4294967295": math.MaxUint32, // exact uint32 boundary
		}
		for in, want := range cases {
			got, ok := parseTconst(in)
			require.True(t, ok, "parseTconst(%q) must succeed", in)
			assert.Equal(t, want, got, "parseTconst(%q)", in)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		// Each of these should return ok=false: missing prefix, missing
		// digits, non-digit body, overflow, raw IMDb NULL, empty input.
		invalid := []string{
			"",
			"t",
			"tt",
			"xt0944947",
			"tx0944947",
			"0944947",
			"tt-1",
			"tt 1",
			"tt0944947x",
			"tt0944947 ",
			`\N`,
			"tt4294967296", // one past uint32 max
			"tt99999999999", // wildly overflows
		}
		for _, in := range invalid {
			_, ok := parseTconst(in)
			assert.False(t, ok, "parseTconst(%q) must return ok=false", in)
		}
	})
}

func TestParseEpisodesJoinAndSort(t *testing.T) {
	titles, err := parseRatings(bytes.NewReader(gzipBytes(t, sampleRatings)))
	require.NoError(t, err)

	episodes, err := parseEpisodes(bytes.NewReader(gzipBytes(t, sampleEpisodes)), titles)
	require.NoError(t, err)

	gotSeries, ok := episodes[mustParseTconst(t, "tt0944947")]
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

	p.snap.Store(snapshotFromMaps(titles, episodes))
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

	t.Run("series rating malformed tconst", func(t *testing.T) {
		// Numeric keys reject inputs that aren't "tt" + decimal digits;
		// we should fall through to ok=false rather than panicking.
		for _, s := range []string{"nope", "tt", "ttabc", "0944947", "tt0944947x"} {
			_, _, ok := p.SeriesRating(s)
			assert.False(t, ok, "SeriesRating(%q) must return ok=false", s)
		}
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

// TestSnapshotBinarySearchBoundaries hits the corner cases that distinguish
// the sorted-slice + binary-search lookup from the previous map shape: misses
// before the first ID, after the last ID, between two adjacent IDs, and at
// the exact min/max IDs in the slice. Boundary lookups must hit exactly; gap
// lookups must miss.
func TestSnapshotBinarySearchBoundaries(t *testing.T) {
	titles := map[uint32]Score{
		1:         {Rating: 5.0, Votes: 10},
		100:       {Rating: 6.0, Votes: 20},
		200:       {Rating: 7.0, Votes: 30},
		1_000_000: {Rating: 9.9, Votes: 1_000_000},
	}
	episodes := map[uint32][]EpisodeScore{
		100: {{Season: 1, Episode: 1, Rating: 6.5, Votes: 5}},
	}

	p := &Provider{}
	p.snap.Store(snapshotFromMaps(titles, episodes))

	// Boundary hits: smallest ID, largest ID, and a middle ID all resolve.
	for _, want := range []struct {
		id     string
		rating float32
	}{
		{"tt0000001", 5.0},
		{"tt100", 6.0},
		{"tt200", 7.0},
		{"tt1000000", 9.9},
	} {
		r, _, ok := p.SeriesRating(want.id)
		require.True(t, ok, "%s must hit", want.id)
		assert.InDelta(t, want.rating, r, 0.001, "%s rating", want.id)
	}

	// Misses: gap below first, between adjacent IDs, just below the largest,
	// and above the largest. The above-max case is the slices.BinarySearchFunc
	// "idx >= len(slice)" branch — covered explicitly because real-data top
	// IDs are far below uint32 max, so production lookups regularly hit it.
	for _, miss := range []string{"tt0", "tt2", "tt99", "tt150", "tt199", "tt201", "tt999999", "tt1000001", "tt4294967295"} {
		_, _, ok := p.SeriesRating(miss)
		assert.False(t, ok, "%s must miss", miss)
	}

	// EpisodeRating boundary: a series ID present in titles but absent from
	// episodes must miss without consulting the inner slice.
	_, _, ok := p.EpisodeRating("tt200", 1, 1)
	assert.False(t, ok, "series with no episode entries must miss")

	// EpisodeRating boundary: present series, present (s,e) tuple — hits.
	r, _, ok := p.EpisodeRating("tt100", 1, 1)
	require.True(t, ok)
	assert.InDelta(t, 6.5, r, 0.001)
}

// TestParseRatingsSkipsMalformedRows — Opt 4 dropped strings.SplitN in favour
// of bytes.Cut, which means lines with the wrong number of \t separators or
// unparseable numeric fields take a different code path than they used to.
// This test pins the behaviour: the parser silently skips broken rows and
// keeps every valid row.
func TestParseRatingsSkipsMalformedRows(t *testing.T) {
	raw := "tconst\taverageRating\tnumVotes\n" +
		"tt100\t8.0\t1000\n" + //                      valid baseline
		"tt101\tnotafloat\t500\n" + //                 ParseFloat fails
		"tt102\t7.5\tnotanint\n" + //                  ParseUint fails
		"tt103\t6.0\n" + //                            short row, only one tab
		"tt104onlyonefield\n" + //                     short row, no tabs
		"tt105\t5.0\t100\n" //                         valid

	titles, err := parseRatings(bytes.NewReader(gzipBytes(t, raw)))
	require.NoError(t, err)

	for _, ok := range []string{"tt100", "tt105"} {
		_, present := titles[mustParseTconst(t, ok)]
		assert.True(t, present, "valid row %s must parse", ok)
	}

	// Each broken row must be skipped — every key absent from the map.
	for _, sk := range []string{"tt101", "tt102", "tt103", "tt104onlyonefield"} {
		id, parseable := parseTconst(sk)
		if !parseable {
			continue // tconst itself didn't parse; absence is implicit
		}
		_, present := titles[id]
		assert.False(t, present, "%s must be skipped", sk)
	}

	assert.Len(t, titles, 2, "exactly two valid rows survive")
}

// TestParseRatingsHandlesNoTrailingNewline — the bufio.Reader.ReadSlice loop
// returns the final line with io.EOF when the file does not end with \n.
// We must still parse that row; previously bufio.Scanner did this for free,
// but the new loop has to handle it explicitly.
func TestParseRatingsHandlesNoTrailingNewline(t *testing.T) {
	raw := "tconst\taverageRating\tnumVotes\n" +
		"tt100\t8.0\t1000\n" +
		"tt101\t9.0\t2000" // intentionally no trailing \n

	titles, err := parseRatings(bytes.NewReader(gzipBytes(t, raw)))
	require.NoError(t, err)

	assert.Len(t, titles, 2, "both rows must parse, including the unterminated last row")
	got, ok := titles[mustParseTconst(t, "tt101")]
	require.True(t, ok)
	assert.InDelta(t, 9.0, got.Rating, 0.001)
	assert.Equal(t, uint32(2000), got.Votes)
}

// TestParseEpisodesSkipsMalformedRows — same shape as the ratings variant,
// covering the four-field episode schema after the bytes-level split.
func TestParseEpisodesSkipsMalformedRows(t *testing.T) {
	titles, err := parseRatings(bytes.NewReader(gzipBytes(t, sampleRatings)))
	require.NoError(t, err)

	raw := "tconst\tparentTconst\tseasonNumber\tepisodeNumber\n" +
		"tt1480055\ttt0944947\t1\t1\n" + //              valid baseline
		"tt1480056\ttt0944947\t1\t2\n" + //              valid baseline
		"tt1480055\ttt0944947\t1\n" + //                 short row, 3 fields
		"tt1480055\ttt0944947\tnotanint\t1\n" + //       Atoi fails on season
		"tt1480055\ttt0944947\t1\tnotanint\n" + //       Atoi fails on episode
		"tt1480055badtconst\ttt0944947\t1\t1\n" + //     parseTconst rejects ep id
		"tt1480055\ttt0944947badparent\t1\t1\n" //       parseTconst rejects parent id (rated ep id, but unparseable parent)

	episodes, err := parseEpisodes(bytes.NewReader(gzipBytes(t, raw)), titles)
	require.NoError(t, err)

	got, ok := episodes[mustParseTconst(t, "tt0944947")]
	require.True(t, ok)
	require.Len(t, got, 2, "only the two well-formed rows must join through")
}

// TestParseRatingsEmptyAndHeaderOnly — both parsers must return an empty,
// non-nil map (and no error) for streams that contain no data rows. Covers
// the io.EOF short-circuit branches in skipHeaderLine: an empty stream and
// a stream containing only the schema header. Real production scenario:
// the upstream IMDb host returns a 0-byte gzip body during an outage.
func TestParseRatingsEmptyAndHeaderOnly(t *testing.T) {
	t.Run("empty stream", func(t *testing.T) {
		titles, err := parseRatings(bytes.NewReader(gzipBytes(t, "")))
		require.NoError(t, err)
		require.NotNil(t, titles)
		assert.Empty(t, titles)
	})
	t.Run("header only", func(t *testing.T) {
		titles, err := parseRatings(bytes.NewReader(gzipBytes(t, "tconst\taverageRating\tnumVotes\n")))
		require.NoError(t, err)
		require.NotNil(t, titles)
		assert.Empty(t, titles)
	})
}

func TestParseEpisodesEmptyAndHeaderOnly(t *testing.T) {
	titles := map[uint32]Score{mustParseTconst(t, "tt1"): {Rating: 8.0, Votes: 100}}
	t.Run("empty stream", func(t *testing.T) {
		episodes, err := parseEpisodes(bytes.NewReader(gzipBytes(t, "")), titles)
		require.NoError(t, err)
		require.NotNil(t, episodes)
		assert.Empty(t, episodes)
	})
	t.Run("header only", func(t *testing.T) {
		episodes, err := parseEpisodes(bytes.NewReader(gzipBytes(t, "tconst\tparentTconst\tseasonNumber\tepisodeNumber\n")), titles)
		require.NoError(t, err)
		require.NotNil(t, episodes)
		assert.Empty(t, episodes)
	})
}

// TestSnapshotConcurrentReadWrite drives the lock-free read path against the
// publish path under -race. The snapshot itself is documented as immutable
// post-publish and reads are atomic.Pointer.Load(); writes are Store(). Any
// future change that mutates a published snapshot or adds non-atomic Provider
// fields shared between reader and refresh paths should fail this test under
// the race detector.
func TestSnapshotConcurrentReadWrite(t *testing.T) {
	titles, err := parseRatings(bytes.NewReader(gzipBytes(t, sampleRatings)))
	require.NoError(t, err)
	episodes, err := parseEpisodes(bytes.NewReader(gzipBytes(t, sampleEpisodes)), titles)
	require.NoError(t, err)
	p := &Provider{}
	p.snap.Store(snapshotFromMaps(titles, episodes))

	const readers = 4
	const iters = 1000
	stop := make(chan struct{})
	done := make(chan struct{}, readers+1)

	for range readers {
		go func() {
			defer func() { done <- struct{}{} }()
			for {
				select {
				case <-stop:
					return
				default:
					_, _, _ = p.SeriesRating("tt0944947")
					_, _, _ = p.EpisodeRating("tt0944947", 1, 1)
				}
			}
		}()
	}

	go func() {
		defer func() { done <- struct{}{} }()
		for range iters {
			p.snap.Store(snapshotFromMaps(titles, episodes))
		}
	}()

	// Let the writer finish, then halt readers.
	<-done
	close(stop)
	for range readers {
		<-done
	}

	r, _, ok := p.SeriesRating("tt0944947")
	require.True(t, ok)
	assert.InDelta(t, 9.2, r, 0.001)
}

// TestParseEpisodesPreallocatesPerParentSlices — Opt 5 asserts that each
// per-parent slice in the parsed output has capacity exactly equal to its
// length. The two-phase parser counts rows per parent on the streaming pass,
// then pre-allocates each per-parent slice to its exact final size and fills
// it. Any future regression that re-introduces append doublings would push
// cap above len here.
func TestParseEpisodesPreallocatesPerParentSlices(t *testing.T) {
	// Use a wider sample so several parents have multiple episodes — that's
	// the case where append-doublings used to leave trailing slack.
	const ratings = "tconst\taverageRating\tnumVotes\n" +
		"tt100\t8.0\t1000\n" +
		"tt101\t8.1\t1000\n" +
		"tt102\t8.2\t1000\n" +
		"tt103\t8.3\t1000\n" +
		"tt104\t8.4\t1000\n" +
		"tt200\t9.0\t2000\n" +
		"tt201\t9.1\t2000\n" +
		"tt202\t9.2\t2000\n"
	const episodes = "tconst\tparentTconst\tseasonNumber\tepisodeNumber\n" +
		"tt100\ttt900\t1\t1\n" +
		"tt101\ttt900\t1\t2\n" +
		"tt102\ttt900\t1\t3\n" +
		"tt103\ttt900\t2\t1\n" +
		"tt104\ttt900\t2\t2\n" +
		"tt200\ttt901\t1\t1\n" +
		"tt201\ttt901\t1\t2\n" +
		"tt202\ttt901\t1\t3\n"

	titles, err := parseRatings(bytes.NewReader(gzipBytes(t, ratings)))
	require.NoError(t, err)
	out, err := parseEpisodes(bytes.NewReader(gzipBytes(t, episodes)), titles)
	require.NoError(t, err)

	require.Len(t, out[mustParseTconst(t, "tt900")], 5)
	require.Len(t, out[mustParseTconst(t, "tt901")], 3)
	for parent, eps := range out {
		assert.Equal(t, len(eps), cap(eps),
			"per-parent slice for parent=%d must be pre-sized: len=%d cap=%d",
			parent, len(eps), cap(eps))
	}
}

// TestSortEpisodesByAirOrder asserts the per-series sort orders by Season
// first, then Episode within the same season, regardless of the order rows
// were emitted by the parser. Exercises both branches of the comparator
// (season-differing and episode-differing) so a regression in either branch
// fails the test.
func TestSortEpisodesByAirOrder(t *testing.T) {
	out := map[uint32][]EpisodeScore{
		1: {
			{Season: 2, Episode: 1, Rating: 7.0},
			{Season: 1, Episode: 3, Rating: 8.0},
			{Season: 1, Episode: 1, Rating: 9.0},
			{Season: 3, Episode: 5, Rating: 6.5},
			{Season: 1, Episode: 2, Rating: 8.5},
			{Season: 2, Episode: 2, Rating: 7.5},
		},
	}
	sortEpisodesByAirOrder(out)

	want := []EpisodeScore{
		{Season: 1, Episode: 1, Rating: 9.0},
		{Season: 1, Episode: 2, Rating: 8.5},
		{Season: 1, Episode: 3, Rating: 8.0},
		{Season: 2, Episode: 1, Rating: 7.0},
		{Season: 2, Episode: 2, Rating: 7.5},
		{Season: 3, Episode: 5, Rating: 6.5},
	}
	assert.Equal(t, want, out[1])
}

// TestParseRatingsHandlesTrailingNewlinesAndCRLF — defensive coverage for
// CRLF-terminated lines and stray blank lines. bufio.Reader.ReadSlice does
// not strip trailing \r, so the parser does it explicitly via trimLineEnd;
// well-formed rows still parse and blank lines must not panic or pollute
// the map.
func TestParseRatingsHandlesTrailingNewlinesAndCRLF(t *testing.T) {
	raw := "tconst\taverageRating\tnumVotes\r\n" +
		"tt1\t8.0\t100\r\n" +
		"\r\n" + // blank line
		"tt2\t7.0\t200\n"
	titles, err := parseRatings(bytes.NewReader(gzipBytes(t, raw)))
	require.NoError(t, err)

	got1, ok := titles[mustParseTconst(t, "tt1")]
	require.True(t, ok)
	assert.InDelta(t, 8.0, got1.Rating, 0.001)
	assert.Equal(t, uint32(100), got1.Votes)

	got2, ok := titles[mustParseTconst(t, "tt2")]
	require.True(t, ok)
	assert.InDelta(t, 7.0, got2.Rating, 0.001)

	// Sanity: the blank line / CRLF stripping leaves no spurious entries.
	assert.Len(t, titles, 2)

	// Sanity: input really did contain a blank CRLF line.
	assert.True(t, strings.Contains(raw, "\r\n\r\n"))
}
