package imdbratings

import (
	"bufio"
	"bytes"
	"cmp"
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
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
)

const (
	ratingsFile = "title.ratings.tsv.gz"
	episodeFile = "title.episode.tsv.gz"

	// IMDb's NULL sentinel.
	tsvNull = `\N`

	// episodeCols is the field count of one row in title.episode.tsv:
	// tconst, parentTconst, seasonNumber, episodeNumber. The parser walks
	// fields by tab index, so this is the loop bound for the inline split.
	// (The ratings parser only has three fields and splits them inline by
	// two bytes.Cut calls, so it doesn't need a named constant.)
	episodeCols = 4

	// Initial map sizing — empirical 2026 figures. Go's map sizes its
	// bucket count to log2(hint / 6.5), giving ~262k buckets for hint
	// 1.5M (~1.7M capacity before rehash). Real row count is ~1.66M, so
	// 1.5M is just under the rehash threshold and avoids growing mid-parse
	// while keeping the build-time allocation small. Bumping to 2M
	// crosses into the next pow-2 bucket count (524k) and ~doubles the
	// map's resident size for no measurable gain.
	initialTitlesCapacity   = 1_500_000
	initialEpisodesCapacity = 250_000

	// bufio.Reader buffer size for the parsers. Default 4 KB is fine for
	// IMDb's ~50-100-byte rows, but a larger buffer reduces syscall count
	// against the gzip reader and trivially absorbs any pathological row
	// length without falling back to the slow ErrBufferFull path.
	readerBufferBytes = 1 << 20

	cacheFileMode = 0o644
	cacheDirMode  = 0o755

	ratingFloatBitSize = 32
	votesUintBitSize   = 32
)

// errNon200 is the static error returned when an HTTP fetch responds with a
// non-2xx status. Used so callers can errors.Is-check it if needed.
var errNon200 = errors.New("non-200 response")

// tsvNullBytes is the byte-slice form of IMDb's NULL sentinel (`\N`), used by
// the parsers to short-circuit rows containing missing fields without
// allocating a string from the line buffer.
//
//nolint:gochecknoglobals // immutable lookup constant; []byte cannot be const
var tsvNullBytes = []byte(tsvNull)

// tabSep is the one-byte TSV field separator. Hoisted to a package var so
// bytes.Cut callers don't reallocate it per row.
//
//nolint:gochecknoglobals // immutable lookup constant; []byte cannot be const
var tabSep = []byte{'\t'}

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

	return snapshotFromMaps(titles, episodes), nil
}

// snapshotFromMaps converts the build-time maps (which we use during parsing
// for O(1) join-side lookups) into the published snapshot's ID-sorted slice
// shape. The map containers are released by the GC after this returns; the
// EpisodeScore slices are reused without copying.
//
// Sort cost is O(N log N) once at the end of build (~45k titles, ~45k series
// in 2026 IMDb data) — negligible against the multi-second parse phase. The
// shape is read-only from this point forward, so the lookup path can binary
// search without any locking or copy-on-read.
func snapshotFromMaps(titles map[uint32]Score, episodes map[uint32][]EpisodeScore) *snapshot {
	titleEntries := make([]titleEntry, 0, len(titles))
	for id, score := range titles {
		titleEntries = append(titleEntries, titleEntry{ID: id, Score: score})
	}
	slices.SortFunc(titleEntries, func(a, b titleEntry) int {
		return cmp.Compare(a.ID, b.ID)
	})

	episodeEntries := make([]seriesEpisodes, 0, len(episodes))
	for id, eps := range episodes {
		episodeEntries = append(episodeEntries, seriesEpisodes{ID: id, Episodes: eps})
	}
	slices.SortFunc(episodeEntries, func(a, b seriesEpisodes) int {
		return cmp.Compare(a.ID, b.ID)
	})

	return &snapshot{
		titles:   titleEntries,
		episodes: episodeEntries,
	}
}

// parseTconst converts an IMDb tconst (e.g. "tt0944947") into its numeric
// form: the decimal digits that follow the "tt" prefix, parsed as uint32.
// Returns ok=false on an empty input, missing "tt" prefix, missing digits,
// any non-digit character, or a value that does not fit in uint32. As of
// 2026 IMDb's largest tconst is well under 2^32, so the bound is comfortable.
//
// Generic over string | []byte so the parser path (which holds []byte slices
// returned by bufio.Reader.ReadSlice) and the request path (which holds the
// caller's string ID) share the same logic without per-call allocations or
// unsafe casts. Both s[i] and len(s) behave identically for the two types,
// so the body is type-agnostic.
func parseTconst[T string | []byte](s T) (uint32, bool) {
	const tconstPrefixLen = 2 // "tt"
	if len(s) <= tconstPrefixLen || s[0] != 't' || s[1] != 't' {
		return 0, false
	}
	var v uint64
	for i := tconstPrefixLen; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*tconstDigitBase + uint64(c-'0')
		if v > math.MaxUint32 {
			return 0, false
		}
	}
	return uint32(v), true
}

const tconstDigitBase = 10

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
func pruneTitlesToSeries(titles map[uint32]Score, episodes map[uint32][]EpisodeScore) map[uint32]Score {
	pruned := make(map[uint32]Score, len(episodes))
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

func parseRatingsFile(path string) (map[uint32]Score, error) {
	//nolint:gosec // path is built from the operator-configured cache dir, not user input
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parseRatings(f)
}

// parseRatings streams a gzipped IMDb ratings TSV into a map keyed by the
// numeric tconst form (see parseTconst). Rows whose tconst doesn't parse as
// a uint32 are skipped — same downstream effect as the historical
// parse-failure path, since unrecognised tconsts could never join anything.
//
// Implementation: bufio.Reader.ReadSlice + bytes-level field splits avoid
// the per-row string allocation that bufio.Scanner.Text() used to make.
// Across ~1.66M rows that allocation is the dominant garbage source for
// this parse pass.
//
// Schema: tconst \t averageRating \t numVotes.
func parseRatings(r io.Reader) (map[uint32]Score, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	br := bufio.NewReaderSize(gz, readerBufferBytes)

	out := make(map[uint32]Score, initialTitlesCapacity)

	if err := skipHeaderLine(br); err != nil {
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		return nil, fmt.Errorf("read ratings header: %w", err)
	}

	for {
		line, err := br.ReadSlice('\n')
		if len(line) > 0 {
			if tconst, score, ok := parseRatingsRow(trimLineEnd(line)); ok {
				out[tconst] = score
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("read ratings: %w", err)
		}
	}
	return out, nil
}

// skipHeaderLine consumes one line from br, returning io.EOF if the stream
// ends before any newline (caller treats that as "empty file, return empty
// map"). Other read errors propagate. The discarded slice is only valid
// until the next ReadSlice call, so we never hold it.
func skipHeaderLine(br *bufio.Reader) error {
	_, err := br.ReadSlice('\n')
	return err
}

// trimLineEnd drops a trailing \n and optional preceding \r from the line
// returned by bufio.Reader.ReadSlice. Operates on a sub-slice — no copy —
// so the result still aliases the bufio buffer and must not be retained
// past the current iteration.
func trimLineEnd(line []byte) []byte {
	n := len(line)
	if n > 0 && line[n-1] == '\n' {
		n--
	}
	if n > 0 && line[n-1] == '\r' {
		n--
	}
	return line[:n]
}

// parseRatingsRow extracts (tconst, Score) from one TSV row, returning
// ok=false for malformed, null, or out-of-range rows so the caller can skip.
// Operates entirely on byte slices aliasing the bufio buffer; the only
// strings we materialise are short numeric fields fed to strconv.
func parseRatingsRow(line []byte) (uint32, Score, bool) {
	if len(line) == 0 {
		return 0, Score{}, false
	}
	tconstField, rest, ok := bytes.Cut(line, tabSep)
	if !ok {
		return 0, Score{}, false
	}
	ratingField, votesField, ok := bytes.Cut(rest, tabSep)
	if !ok {
		return 0, Score{}, false
	}
	if bytes.Equal(tconstField, tsvNullBytes) ||
		bytes.Equal(ratingField, tsvNullBytes) ||
		bytes.Equal(votesField, tsvNullBytes) {
		return 0, Score{}, false
	}
	id, ok := parseTconst(tconstField)
	if !ok {
		return 0, Score{}, false
	}
	rating64, err := strconv.ParseFloat(string(ratingField), ratingFloatBitSize)
	if err != nil {
		return 0, Score{}, false
	}
	votes64, err := strconv.ParseUint(string(votesField), 10, votesUintBitSize)
	if err != nil {
		return 0, Score{}, false
	}
	return id, Score{
		Rating: float32(rating64),
		Votes:  uint32(votes64),
	}, true
}

func parseEpisodesFile(path string, titles map[uint32]Score) (map[uint32][]EpisodeScore, error) {
	//nolint:gosec // path is built from the operator-configured cache dir, not user input
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parseEpisodes(f, titles)
}

// parseEpisodes streams a gzipped IMDb episode TSV and joins each row against
// the titles map (keyed by numeric episode tconst), accumulating per-series
// sorted slices. Rows with missing fields, no rating in titles, or
// out-of-range season/episode numbers are dropped.
//
// Implementation matches parseRatings: bufio.Reader.ReadSlice + bytes-level
// field splits, so the per-row sc.Text() allocation is gone. Episode parse
// is the larger of the two passes (~9M rows in 2026) so this is where the
// allocation savings show up most.
//
// Schema: tconst \t parentTconst \t seasonNumber \t episodeNumber.
func parseEpisodes(r io.Reader, titles map[uint32]Score) (map[uint32][]EpisodeScore, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	br := bufio.NewReaderSize(gz, readerBufferBytes)

	out := make(map[uint32][]EpisodeScore, initialEpisodesCapacity)

	if err := skipHeaderLine(br); err != nil {
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		return nil, fmt.Errorf("read episodes header: %w", err)
	}

	for {
		line, err := br.ReadSlice('\n')
		if len(line) > 0 {
			if parent, score, ok := parseEpisodeRow(trimLineEnd(line), titles); ok {
				out[parent] = append(out[parent], score)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("read episodes: %w", err)
		}
	}

	sortEpisodesByAirOrder(out)
	return out, nil
}

// sortEpisodesByAirOrder sorts each per-series slice by (Season, Episode) so
// EpisodeRating's two-level binary search has a deterministic inner order.
// Pulled out of parseEpisodes purely to keep that function under the
// project's cyclomatic-complexity budget.
func sortEpisodesByAirOrder(out map[uint32][]EpisodeScore) {
	for _, eps := range out {
		sort.Slice(eps, func(i, j int) bool {
			if eps[i].Season != eps[j].Season {
				return eps[i].Season < eps[j].Season
			}
			return eps[i].Episode < eps[j].Episode
		})
	}
}

// parseEpisodeRow extracts (parentTconst, EpisodeScore) from one TSV row.
// Returns ok=false on malformed rows, unparseable tconsts, missing
// rating-side join keys, or season/episode numbers that don't fit in int16.
// Each field is a sub-slice of the input line (no copies); only the
// season/episode numeric fields are converted to short-lived strings for
// strconv.Atoi.
func parseEpisodeRow(line []byte, titles map[uint32]Score) (uint32, EpisodeScore, bool) {
	if len(line) == 0 {
		return 0, EpisodeScore{}, false
	}
	var fields [episodeCols][]byte
	rest := line
	for k := range episodeCols - 1 {
		i := bytes.IndexByte(rest, '\t')
		if i < 0 {
			return 0, EpisodeScore{}, false
		}
		fields[k] = rest[:i]
		rest = rest[i+1:]
	}
	fields[episodeCols-1] = rest
	for _, f := range fields {
		if bytes.Equal(f, tsvNullBytes) {
			return 0, EpisodeScore{}, false
		}
	}
	episodeID, ok := parseTconst(fields[0])
	if !ok {
		return 0, EpisodeScore{}, false
	}
	score, ok := titles[episodeID]
	if !ok {
		return 0, EpisodeScore{}, false
	}
	parentID, ok := parseTconst(fields[1])
	if !ok {
		return 0, EpisodeScore{}, false
	}
	season, episode, ok := parseSeasonEpisode(fields[2], fields[3])
	if !ok {
		return 0, EpisodeScore{}, false
	}
	return parentID, EpisodeScore{
		Season:  season,
		Episode: episode,
		Rating:  score.Rating,
		Votes:   score.Votes,
	}, true
}

// parseSeasonEpisode parses the season+episode columns into bounded int16s,
// returning ok=false on parse failure or out-of-range numbers. strconv.Atoi
// only accepts string, so we materialise short (1-3 byte) strings here; the
// allocation cost is dwarfed by the per-line allocation we're saving.
func parseSeasonEpisode(seasonField, episodeField []byte) (int16, int16, bool) {
	seasonI, err := strconv.Atoi(string(seasonField))
	if err != nil || seasonI < math.MinInt16 || seasonI > math.MaxInt16 {
		return 0, 0, false
	}
	episodeI, err := strconv.Atoi(string(episodeField))
	if err != nil || episodeI < math.MinInt16 || episodeI > math.MaxInt16 {
		return 0, 0, false
	}
	//nolint:gosec // bounds checked above
	return int16(seasonI), int16(episodeI), true
}
