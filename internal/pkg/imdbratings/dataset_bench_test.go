package imdbratings

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// Synthetic bench shape: chosen to roughly match real 2026 IMDb data so the
// allocator/GC pressure is comparable. Real ratings file is ~1.66M rows; real
// episode file is ~9M rows of which ~3M join against ratings. The synthetic
// generator uses a fixed RNG seed so results are reproducible across runs.
const (
	syntheticRatingsRows = 1_660_000
	syntheticEpisodeRows = 9_000_000
)

// reportHeapInUseMB samples runtime.MemStats.HeapInuse after a forced GC and
// reports it as a custom benchmark metric in MB. The forced GC makes the
// number reflect retained-after-build memory rather than transient noise.
func reportHeapInUseMB(b *testing.B) {
	b.Helper()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	b.ReportMetric(float64(m.HeapInuse)/1024/1024, "MB-heap")
}

// generateSyntheticDatasets builds an in-memory pair of gzipped TSVs with
// shape and join cardinality close to the real IMDb dumps. The generation is
// deterministic for a given (ratingsRows, episodeRows) so benchstat can
// compare runs apples-to-apples.
//
// Layout choices:
//   - ratedTconsts is the set of IDs that show up in ratings. About a third of
//     episode rows reuse one of these IDs as their episode tconst, so the
//     parser-side join produces ~episodeRows/3 hits — matching the real
//     ~3M/9M ratio.
//   - Parent (series) IDs are drawn as a random subset of ratedTconsts sized
//     roughly 1/30th of ratingsRows, mirroring the 45k-series-per-1.66M-titles
//     real shape. Drawing parents from rated IDs (rather than a disjoint
//     range) ensures pruneTitlesToSeries has real work to do — otherwise
//     every parent fails the title lookup and the published titles slice is
//     empty, making the BuildSnapshot bench unrepresentative.
func generateSyntheticDatasets(tb testing.TB, ratingsRows, episodeRows int) (ratingsGz, episodesGz []byte) {
	tb.Helper()
	rng := rand.New(rand.NewSource(1))

	ratedTconsts := make([]uint32, 0, ratingsRows)
	used := make(map[uint32]struct{}, ratingsRows)
	idSpan := ratingsRows * 2

	var ratingsBuf bytes.Buffer
	rgz := gzip.NewWriter(&ratingsBuf)
	_, err := rgz.Write([]byte("tconst\taverageRating\tnumVotes\n"))
	require.NoError(tb, err)
	for len(ratedTconsts) < ratingsRows {
		id := uint32(rng.Intn(idSpan)) + 1
		if _, ok := used[id]; ok {
			continue
		}
		used[id] = struct{}{}
		ratedTconsts = append(ratedTconsts, id)
		rating := 1.0 + rng.Float64()*9.0
		votes := rng.Intn(2_000_000) + 1
		_, err := fmt.Fprintf(rgz, "tt%07d\t%.1f\t%d\n", id, rating, votes)
		require.NoError(tb, err)
	}
	require.NoError(tb, rgz.Close())

	numSeries := min(max(ratingsRows/30, 100), len(ratedTconsts))
	parents := make([]uint32, numSeries)
	for i := range parents {
		parents[i] = ratedTconsts[rng.Intn(len(ratedTconsts))]
	}

	var episodesBuf bytes.Buffer
	egz := gzip.NewWriter(&episodesBuf)
	_, err = egz.Write([]byte("tconst\tparentTconst\tseasonNumber\tepisodeNumber\n"))
	require.NoError(tb, err)
	episodeBase := uint32(idSpan)*3 + 1
	for range episodeRows {
		var epTconst uint32
		if rng.Intn(3) == 0 {
			epTconst = ratedTconsts[rng.Intn(len(ratedTconsts))]
		} else {
			epTconst = episodeBase + uint32(rng.Intn(idSpan*2))
		}
		parent := parents[rng.Intn(len(parents))]
		season := rng.Intn(20) + 1
		episode := rng.Intn(30) + 1
		_, err := fmt.Fprintf(egz, "tt%07d\ttt%07d\t%d\t%d\n", epTconst, parent, season, episode)
		require.NoError(tb, err)
	}
	require.NoError(tb, egz.Close())

	return ratingsBuf.Bytes(), episodesBuf.Bytes()
}

// titlesFromGz parses a gzipped ratings blob into the titles map; used by
// BenchmarkParseEpisodes_Synthetic to set up its join target outside the
// timed loop.
func titlesFromGz(tb testing.TB, gz []byte) map[uint32]Score {
	tb.Helper()
	titles, err := parseRatings(bytes.NewReader(gz))
	require.NoError(tb, err)
	return titles
}

// writeGzToTempFile materializes a gzipped blob to disk so
// buildSnapshotFromFiles can stream it. Cleanup is registered against tb.
func writeGzToTempFile(tb testing.TB, dir, name string, data []byte) string {
	tb.Helper()
	path := filepath.Join(dir, name)
	require.NoError(tb, os.WriteFile(path, data, 0o644))
	return path
}

func BenchmarkParseRatings_Synthetic(b *testing.B) {
	ratingsGz, _ := generateSyntheticDatasets(b, syntheticRatingsRows, 0)
	b.ReportAllocs()
	var titles map[uint32]Score
	for b.Loop() {
		t, err := parseRatings(bytes.NewReader(ratingsGz))
		if err != nil {
			b.Fatalf("parseRatings: %v", err)
		}
		titles = t
	}
	b.StopTimer()
	// reportHeapInUseMB runs GC; KeepAlive must be after it so the map is
	// still reachable from a live local across the GC, otherwise the compiler
	// can treat `titles` as dead and the metric undercounts.
	reportHeapInUseMB(b)
	runtime.KeepAlive(titles)
}

func BenchmarkParseEpisodes_Synthetic(b *testing.B) {
	ratingsGz, episodesGz := generateSyntheticDatasets(b, syntheticRatingsRows, syntheticEpisodeRows)
	titles := titlesFromGz(b, ratingsGz)
	b.ReportAllocs()
	var episodes map[uint32][]EpisodeScore
	for b.Loop() {
		e, err := parseEpisodes(bytes.NewReader(episodesGz), titles)
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

func BenchmarkBuildSnapshot_Synthetic(b *testing.B) {
	ratingsGz, episodesGz := generateSyntheticDatasets(b, syntheticRatingsRows, syntheticEpisodeRows)
	dir := b.TempDir()
	ratingsPath := writeGzToTempFile(b, dir, ratingsFile, ratingsGz)
	episodePath := writeGzToTempFile(b, dir, episodeFile, episodesGz)
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
