//go:build imdbbench

package imdbratings

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// realBenchDataDir returns the directory that holds the real, gzipped IMDb
// TSVs used by the _Real bench family. The default — `../../../var/imdb`
// relative to this test file's package — matches the gitignored cache that
// `make docker-up` populates. Override with LAZYSOAP_BENCH_DATA_DIR.
//
// Operator opted in by passing -tags imdbbench, so a missing file is a hard
// failure rather than a silent skip — otherwise running `make bench-real`
// against a stale or empty data dir would silently produce no measurement.
func realBenchDataDir() string {
	if v := os.Getenv("LAZYSOAP_BENCH_DATA_DIR"); v != "" {
		return v
	}
	return filepath.Join("..", "..", "..", "var", "imdb")
}

func mustReadRealFile(tb testing.TB, name string) string {
	tb.Helper()
	path := filepath.Join(realBenchDataDir(), name)
	st, err := os.Stat(path)
	if err != nil {
		tb.Fatalf("real bench data missing at %s: %v (set LAZYSOAP_BENCH_DATA_DIR or populate the default)", path, err)
	}
	if st.Size() == 0 {
		tb.Fatalf("real bench data file is empty: %s", path)
	}
	return path
}

func mustOpenRealFile(tb testing.TB, name string) *os.File {
	tb.Helper()
	path := mustReadRealFile(tb, name)
	f, err := os.Open(path) //nolint:gosec // operator-controlled bench fixture path
	if err != nil {
		tb.Fatalf("open %s: %v", path, err)
	}
	return f
}

func BenchmarkParseRatings_Real(b *testing.B) {
	path := mustReadRealFile(b, ratingsFile)
	b.ReportAllocs()
	var titles map[uint32]Score
	for b.Loop() {
		f, err := os.Open(path) //nolint:gosec // operator-controlled bench fixture path
		if err != nil {
			b.Fatalf("open: %v", err)
		}
		t, err := parseRatings(f)
		_ = f.Close()
		if err != nil {
			b.Fatalf("parseRatings: %v", err)
		}
		titles = t
	}
	b.StopTimer()
	reportHeapInUseMB(b)
	runtime.KeepAlive(titles)
}

func BenchmarkParseEpisodes_Real(b *testing.B) {
	titles := buildRealTitles(b)
	episodePath := mustReadRealFile(b, episodeFile)
	b.ReportAllocs()
	var episodes map[uint32][]EpisodeScore
	for b.Loop() {
		f, err := os.Open(episodePath) //nolint:gosec // operator-controlled bench fixture path
		if err != nil {
			b.Fatalf("open: %v", err)
		}
		e, err := parseEpisodes(f, titles)
		_ = f.Close()
		if err != nil {
			b.Fatalf("parseEpisodes: %v", err)
		}
		episodes = e
	}
	b.StopTimer()
	reportHeapInUseMB(b)
	runtime.KeepAlive(episodes)
	runtime.KeepAlive(titles)
}

func BenchmarkBuildSnapshot_Real(b *testing.B) {
	ratingsPath := mustReadRealFile(b, ratingsFile)
	episodePath := mustReadRealFile(b, episodeFile)
	ctx := context.Background()
	b.ReportAllocs()
	var snap *snapshot
	for b.Loop() {
		s, err := buildSnapshotFromFiles(ctx, ratingsPath, episodePath)
		if err != nil {
			b.Fatalf("buildSnapshotFromFiles: %v", err)
		}
		snap = s
	}
	b.StopTimer()
	reportHeapInUseMB(b)
	runtime.KeepAlive(snap)
}

func buildRealTitles(tb testing.TB) map[uint32]Score {
	tb.Helper()
	f := mustOpenRealFile(tb, ratingsFile)
	defer func() { _ = f.Close() }()
	titles, err := parseRatings(f)
	if err != nil {
		tb.Fatalf("buildRealTitles: %v", err)
	}
	return titles
}
