package imdbratings

import (
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
)

const (
	ratingsFile = "title.ratings.tsv.gz"
	episodeFile = "title.episode.tsv.gz"

	// IMDb's NULL sentinel.
	tsvNull = `\N`

	// Column counts for the two TSVs we read. Keeping them named avoids the
	// magic-number lint and documents the wire schema next to the parsers.
	ratingsCols = 3 // tconst, averageRating, numVotes
	episodeCols = 4 // tconst, parentTconst, seasonNumber, episodeNumber

	// Initial map sizing — empirical 2026 figures. Go's map sizes its
	// bucket count to log2(hint / 6.5), giving ~262k buckets for hint
	// 1.5M (~1.7M capacity before rehash). Real row count is ~1.66M, so
	// 1.5M is just under the rehash threshold and avoids growing mid-parse
	// while keeping the build-time allocation small. Bumping to 2M
	// crosses into the next pow-2 bucket count (524k) and ~doubles the
	// map's resident size for no measurable gain.
	initialTitlesCapacity   = 1_500_000
	initialEpisodesCapacity = 250_000

	// Bumped scanner buffer; default 64 KB has bitten people on edge cases.
	scannerBufferBytes = 1 << 20

	cacheFileMode = 0o644
	cacheDirMode  = 0o755

	ratingFloatBitSize = 32
	votesUintBitSize   = 32
)

// errNon200 is the static error returned when an HTTP fetch responds with a
// non-2xx status. Used so callers can errors.Is-check it if needed.
var errNon200 = errors.New("non-200 response")

// buildSnapshot downloads the two TSV gzip dumps (or reuses existing on-disk
// caches when the network call fails) and delegates to buildSnapshotFromFiles
// to parse and join them. Splitting the I/O from the parse pipeline lets
// benches and unit tests drive the parser without an HTTP round-trip.
func buildSnapshot(ctx context.Context, client *http.Client, cfg Config) (*snapshot, error) {
	if err := os.MkdirAll(cfg.CacheDir, cacheDirMode); err != nil {
		return nil, fmt.Errorf("mkdir cache dir: %w", err)
	}

	ratingsPath := filepath.Join(cfg.CacheDir, ratingsFile)
	episodePath := filepath.Join(cfg.CacheDir, episodeFile)

	if err := refreshFile(ctx, client, cfg.DatasetsBaseURL+"/"+ratingsFile, ratingsPath, cfg.RefreshInterval); err != nil {
		return nil, fmt.Errorf("refresh ratings: %w", err)
	}
	if err := refreshFile(ctx, client, cfg.DatasetsBaseURL+"/"+episodeFile, episodePath, cfg.RefreshInterval); err != nil {
		return nil, fmt.Errorf("refresh episode: %w", err)
	}

	return buildSnapshotFromFiles(ctx, ratingsPath, episodePath)
}

// buildSnapshotFromFiles parses the two on-disk gzipped TSVs streaming, joins
// them, sorts each per-series episode slice, and returns the resulting
// snapshot. The ctx parameter is reserved for future cancellation hooks; it
// is not consulted today since the parsers are CPU-bound and cheap to let
// run to completion.
func buildSnapshotFromFiles(_ context.Context, ratingsPath, episodePath string) (*snapshot, error) {
	titles, err := parseRatingsFile(ratingsPath)
	if err != nil {
		return nil, fmt.Errorf("parse ratings: %w", err)
	}

	episodes, err := parseEpisodesFile(episodePath, titles)
	if err != nil {
		return nil, fmt.Errorf("parse episodes: %w", err)
	}

	titles = pruneTitlesToSeries(titles, episodes)

	return &snapshot{
		titles:   titles,
		episodes: episodes,
	}, nil
}

// pruneTitlesToSeries shrinks the titles map to only entries that appear as
// a parentTconst in the joined episodes map. After this point the only
// consumer of titles is SeriesRating(seriesTconst), so the dropped entries —
// movies, individual episodes, shorts, video games, etc. — are pure
// retained-memory waste. On 2026 IMDb data this drops the map from ~1.66M
// down to ~45k entries; together with the tconst-string objects it roots,
// it frees roughly 80 MB of resident heap.
//
// Edge case (intentional): series that are rated in title.ratings.tsv.gz but
// have no rated episodes in title.episode.tsv.gz lose their series-level
// IMDb rating and fall through to TMDB. This is statistically tiny in
// practice — most rated series have at least one rated episode — and the
// fallback path is the same one we use for series with no IMDb mapping at
// all, so the failure mode is invisible to the client.
func pruneTitlesToSeries(titles map[string]Score, episodes map[string][]EpisodeScore) map[string]Score {
	pruned := make(map[string]Score, len(episodes))
	for seriesTconst := range episodes {
		if score, ok := titles[seriesTconst]; ok {
			pruned[seriesTconst] = score
		}
	}
	return pruned
}

// refreshFile attempts to update the on-disk cache at destPath from url, but
// only when needed:
//
//   - If destPath exists and is younger than maxAge, the download is
//     skipped (the cache is still fresh).
//   - Otherwise we attempt the download. On success we replace the cache.
//   - On download failure with a stale cache present, we log a warning and
//     keep using the stale file — better to serve last-known IMDb ratings
//     than fail open to TMDB during a transient outage.
//   - On download failure with no cache present, we log the error and
//     surface it; the caller will skip the snapshot publish for this cycle.
//
// maxAge=0 means "no refresh configured" — any cached file is treated as
// authoritative on subsequent boots.
func refreshFile(ctx context.Context, client *http.Client, url, destPath string, maxAge time.Duration) error {
	if age, fresh := cachedFileAge(destPath, maxAge); fresh {
		logger.Info(ctx, "imdb dataset cache hit, skipping download",
			"url", url, "path", destPath, "age", age, "max_age", maxAge,
		)
		return nil
	}

	if err := download(ctx, client, url, destPath); err != nil {
		if _, statErr := os.Stat(destPath); statErr == nil {
			logger.Warn(ctx, "imdb dataset download failed, falling back to cached file",
				"url", url, "path", destPath, "err", err,
			)
			return nil
		}
		logger.Error(ctx, "imdb dataset download failed and no cache is available",
			"url", url, "path", destPath, "err", err,
		)
		return err
	}

	logger.Info(ctx, "imdb dataset downloaded", "url", url, "path", destPath)
	return nil
}

// cachedFileAge returns the file's age (time since last modification) and
// whether the cache should be considered fresh given maxAge. A missing or
// unreadable file is reported as fresh=false. maxAge<=0 means "treat any
// existing cache as authoritative" — useful when the operator has set
// LAZYSOAP_IMDB_REFRESH_INTERVAL=0 to disable periodic refresh entirely.
func cachedFileAge(path string, maxAge time.Duration) (time.Duration, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	age := time.Since(info.ModTime())
	if maxAge <= 0 {
		return age, true
	}
	return age, age < maxAge
}

func download(ctx context.Context, client *http.Client, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %w (status %d)", url, errNon200, resp.StatusCode)
	}

	tmpPath := destPath + ".tmp"
	//nolint:gosec // path is built from the operator-configured cache dir, not user input
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, cacheFileMode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, destPath)
}

func parseRatingsFile(path string) (map[string]Score, error) {
	//nolint:gosec // path is built from the operator-configured cache dir, not user input
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parseRatings(f)
}

// parseRatings streams a gzipped IMDb ratings TSV into a map.
//
// Schema: tconst \t averageRating \t numVotes.
func parseRatings(r io.Reader) (map[string]Score, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, scannerBufferBytes), scannerBufferBytes)

	out := make(map[string]Score, initialTitlesCapacity)

	if !sc.Scan() { // skip header
		return out, sc.Err()
	}

	for sc.Scan() {
		tconst, score, ok := parseRatingsRow(sc.Text())
		if !ok {
			continue
		}
		out[tconst] = score
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan ratings: %w", err)
	}
	return out, nil
}

// parseRatingsRow extracts (tconst, Score) from one TSV row, returning
// ok=false for malformed, null, or out-of-range rows so the caller can skip.
func parseRatingsRow(line string) (string, Score, bool) {
	parts := strings.SplitN(line, "\t", ratingsCols)
	if len(parts) != ratingsCols {
		return "", Score{}, false
	}
	if parts[0] == tsvNull || parts[1] == tsvNull || parts[2] == tsvNull {
		return "", Score{}, false
	}
	rating64, err := strconv.ParseFloat(parts[1], ratingFloatBitSize)
	if err != nil {
		return "", Score{}, false
	}
	votes64, err := strconv.ParseUint(parts[2], 10, votesUintBitSize)
	if err != nil {
		return "", Score{}, false
	}
	return parts[0], Score{
		Rating: float32(rating64),
		Votes:  uint32(votes64),
	}, true
}

func parseEpisodesFile(path string, titles map[string]Score) (map[string][]EpisodeScore, error) {
	//nolint:gosec // path is built from the operator-configured cache dir, not user input
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parseEpisodes(f, titles)
}

// parseEpisodes streams a gzipped IMDb episode TSV and joins each row against
// the titles map (keyed by episode tconst), accumulating per-series sorted
// slices. Rows with missing fields, no rating in titles, or out-of-range
// season/episode numbers are dropped.
//
// Schema: tconst \t parentTconst \t seasonNumber \t episodeNumber.
func parseEpisodes(r io.Reader, titles map[string]Score) (map[string][]EpisodeScore, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, scannerBufferBytes), scannerBufferBytes)

	out := make(map[string][]EpisodeScore, initialEpisodesCapacity)

	if !sc.Scan() { // skip header
		return out, sc.Err()
	}

	for sc.Scan() {
		parent, score, ok := parseEpisodeRow(sc.Text(), titles)
		if !ok {
			continue
		}
		out[parent] = append(out[parent], score)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan episodes: %w", err)
	}

	for _, eps := range out {
		sort.Slice(eps, func(i, j int) bool {
			if eps[i].Season != eps[j].Season {
				return eps[i].Season < eps[j].Season
			}
			return eps[i].Episode < eps[j].Episode
		})
	}
	return out, nil
}

// parseEpisodeRow extracts (parentTconst, EpisodeScore) from one TSV row.
// Returns ok=false on malformed rows, missing rating-side join keys, or
// season/episode numbers that don't fit in int16.
func parseEpisodeRow(line string, titles map[string]Score) (string, EpisodeScore, bool) {
	parts := strings.SplitN(line, "\t", episodeCols)
	if !validEpisodeRowParts(parts) {
		return "", EpisodeScore{}, false
	}
	score, ok := titles[parts[0]]
	if !ok {
		return "", EpisodeScore{}, false
	}
	season, episode, ok := parseSeasonEpisode(parts[2], parts[3])
	if !ok {
		return "", EpisodeScore{}, false
	}
	return parts[1], EpisodeScore{
		Season:  season,
		Episode: episode,
		Rating:  score.Rating,
		Votes:   score.Votes,
	}, true
}

// validEpisodeRowParts checks that we got the right column count and that no
// required column is IMDb's NULL sentinel.
func validEpisodeRowParts(parts []string) bool {
	if len(parts) != episodeCols {
		return false
	}
	return !slices.Contains(parts, tsvNull)
}

// parseSeasonEpisode parses the season+episode columns into bounded int16s,
// returning ok=false on parse failure or out-of-range numbers.
func parseSeasonEpisode(seasonStr, episodeStr string) (int16, int16, bool) {
	seasonI, err := strconv.Atoi(seasonStr)
	if err != nil || seasonI < math.MinInt16 || seasonI > math.MaxInt16 {
		return 0, 0, false
	}
	episodeI, err := strconv.Atoi(episodeStr)
	if err != nil || episodeI < math.MinInt16 || episodeI > math.MaxInt16 {
		return 0, 0, false
	}
	//nolint:gosec // bounds checked above
	return int16(seasonI), int16(episodeI), true
}
