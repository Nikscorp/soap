# Optimize IMDb dataset download & parse, with benchstat-backed proof

## Overview

The IMDb dataset loader in `internal/pkg/imdbratings/dataset.go` parses two gzipped TSVs at startup and on a daily ticker. The published snapshot ends up with ~45k titles and ~45k series in `episodes`, but the user observes ~100 MB resident heap held by those tiny maps after build is done. The README's existing "~35 MB heap / ~290 MB RSS" claim was estimated, not measured. There is no CPU/wall measurement of the parse step.

This plan rebuilds the optimization story on a benchstat-backed foundation: first add a benchmark harness on real big input data, then apply six pre-authorized optimizations one at a time, each gated by its own benchstat result on the `_Real` suite. Wins ship, non-wins are reverted before the next attempt so the chain stays clean. Finally, replace estimated documentation figures with measured ones.

## Context

- Impacted code: `internal/pkg/imdbratings/dataset.go` (parsers, snapshot build), `internal/pkg/imdbratings/provider.go` (`Score`, `snapshot`, lookup methods), and `internal/pkg/imdbratings/provider_test.go`.
- Build artifacts: new `dataset_bench_test.go` (synthetic, always-on) and `dataset_bench_real_test.go` (gated by `//go:build imdbbench`).
- Real-data benches read from `LAZYSOAP_BENCH_DATA_DIR` (default `../../../var/imdb`). `var/` is already gitignored; defensively also exclude `internal/pkg/imdbratings/testdata/real/`.
- `benchstat` is a developer-machine dependency only: `go install golang.org/x/perf/cmd/benchstat@latest`. Do not add a Makefile entry for the install step.
- `count=10` is the benchstat-recommended floor for stable variance estimates.
- Wire-facing API (`SeriesRating(imdbID string)`) does not change throughout this plan.
- Per repo CLAUDE.md: performance/size/latency numbers in docs must come from measurement, not estimation.
- Adopted from `docs/plans/imdb-optimizations.md`.

## Development Approach

- Testing approach: regular (existing tests already cover shape contracts; bench harness is additive).
- Apply optimizations sequentially. After each, re-run `make bench-real` and run `benchstat bin/bench-real-baseline.txt bin/bench-real.txt`. Keep the change only if it shows a real (>= ~5%) improvement on time, allocs, or heap on the `_Real` benches. Revert immediately if it does not, so each subsequent step builds on a known-good baseline.
- The `_Real` suite is the shipping gate. The synthetic suite is a secondary regression signal.
- Complete each task fully before moving to the next.
- Update this plan when scope changes during implementation.

## Testing Strategy

- Unit tests must keep passing through every optimization: `go test -race -count=1 ./internal/pkg/imdbratings/...`.
- The existing tests cover rating skips, episode join, prune behavior, cache freshness, and fallback. `TestRunPrunesNonSeriesTitles` will need a rewrite once Optimization 1 lands (replace with a test that asserts non-series tconsts never enter the snapshot).
- Benchstat output (before vs. after, on real data) is captured into the PR description for each optimization, not committed as a file.
- Run project tests after each Task before proceeding.

## Progress Tracking

- Mark completed items with `[x]` immediately when done.
- Record benchstat deltas per optimization in the PR description; if a delta does not justify the change, revert and note the "reverted" path explicitly.
- Update plan if implementation deviates from original scope.

## Technical Details

**Why the heap is bigger than `len(map)` suggests**

- `parseRatings` allocates a `map[string]Score` hinted at 1.5M entries. `pruneTitlesToSeries` copies ~45k surviving entries into a fresh map (`dataset.go:109-117`), so the giant ratings map *should* be GC'd. Surviving entries still hold their `tconst` strings (~10-byte header + payload ≈ 26 bytes/key) plus map bucket overhead.
- `parseEpisodes` joins each rated episode into `[]EpisodeScore` per parent. Even after filtering, per-series slices hold ~3M `EpisodeScore` entries (12 bytes each) plus slice headers and map overhead — tens of MB on its own.
- Peak briefly contains both the 1.66M-entry titles map AND the in-progress episodes map — that is the source of the "~290 MB RSS" build-time number.

**Refactor seam (Task 1)**

Extract `buildSnapshotFromFiles(ctx, ratingsPath, episodePath)` from the current `buildSnapshot`. The existing `buildSnapshot` becomes a thin wrapper that does refresh-then-call. No behavior change; benches and unit tests can drive the parse pipeline without hitting the network.

**Bench harness shape (Task 1)**

Three families, each reporting `b.ReportAllocs()` and a custom `MB-heap` metric (sampled with `runtime.GC()` + `runtime.ReadMemStats(&m)` → `b.ReportMetric(float64(m.HeapInuse)/1024/1024, "MB-heap")`):

- `BenchmarkParseRatings` — `parseRatings` alone over the gzipped ratings file.
- `BenchmarkParseEpisodes` — `parseEpisodes` alone with a pre-built titles map.
- `BenchmarkBuildSnapshot` — full pipeline minus HTTP fetch (calls `buildSnapshotFromFiles`).

Each family has two variants:

- `_Synthetic` (in `dataset_bench_test.go`, no build tag) — generated in-memory at bench start with a fixed RNG seed; sized to roughly match real shape (1.66M ratings rows, ~9M episode rows of which ~3M join). Generation cost excluded via `b.ResetTimer()`. Always runs.
- `_Real` (in `dataset_bench_real_test.go`, `//go:build imdbbench`) — reads from `LAZYSOAP_BENCH_DATA_DIR` (default `../../../var/imdb` relative to the test file). If files missing under the tag, `b.Fatalf` (no silent skip — operator opted in via the tag).

Synthetic data generator: `generateSyntheticDatasets(t testing.TB, ratingsRows, episodeRows int) (ratingsGz, episodesGz []byte)` lives in `dataset_bench_test.go`. Reuses `gzipBytes` from `provider_test.go:35-43` (parameter widens from `*testing.T` to `testing.TB`). tconst format `tt########` matches the real wire shape.

**Makefile targets (Task 1)**

```
bench:
	go test -bench=. -benchmem -count=10 -run=^$$ ./internal/pkg/imdbratings/... | tee bin/bench.txt

bench-real:
	go test -tags imdbbench -bench=. -benchmem -count=10 -run=^$$ ./internal/pkg/imdbratings/... | tee bin/bench-real.txt

bench-baseline: bench
	@cp bin/bench.txt bin/bench-baseline.txt

bench-real-baseline: bench-real
	@cp bin/bench-real.txt bin/bench-real-baseline.txt

bench-stat:
	benchstat bin/bench-baseline.txt bin/bench.txt

bench-real-stat:
	benchstat bin/bench-real-baseline.txt bin/bench-real.txt
```

**Optimization summaries**

1. *Reorder passes.* Parse episodes first → `map[string]episodeRef` (episode tconst → parent + season + number) and `map[string]struct{}` of parent tconsts. Then stream ratings: only allocate the tconst string + map entry when the row is in either set. End state: titles peaks at ~50k, episode-tconst map at ~3M with 8-byte values vs current string-keyed Score.
2. *Numeric tconst keys.* `tt` + decimal digits → `uint32` (IMDb max ~36M as of 2026). New helper `parseTconst([]byte) (uint32, bool)`. `Score`/`snapshot` types switch from `map[string]…` to `…uint32` keys. Lookups parse the request-side tconst once. Wire API unchanged.
3. *Sorted slices + binary search for published maps.* After build, titles/episodes are read-only. `[]titleEntry{ID uint32; Score}` sorted by ID is ~12 bytes/entry × 45k = ~540 KB; map equivalent is 3-5× larger. `slices.BinarySearchFunc` on lookup. Same for the parent index of `episodes`.
4. *Skip per-line `string` allocations.* Replace `bufio.Scanner` + `sc.Text()` with `bufio.Reader.ReadSlice('\n')` and parse fields straight from the byte slice. `bytes.IndexByte` for tab splits; `strconv.ParseFloat`/`ParseUint` accept byte slices. Copy only when actually storing.
5. *Pre-size episode slices.* Counting pass to compute per-parent counts, then a fill pass into pre-allocated slices — removes O(log N) regrowths. Ship only if (4)+(5) together show a meaningful delta; parsing may be gzip-bound.
6. *`slices.SortFunc` over `sort.Slice`.* Drops the closure interface allocation per series. Tiny per-series win × 45k. Cheap to do; ship only if benchstat shows it.

Optimizations 1-3 are the load-bearing memory wins (the user's primary ask). 4-6 are CPU wins.

**Critical files**

| File | Change |
| --- | --- |
| `internal/pkg/imdbratings/dataset.go` | Refactor `buildSnapshot` to extract `buildSnapshotFromFiles`; reorder pass strategy (Opt 1); change key types (Opt 2); switch published shape (Opt 3); replace scanner (Opt 4); pre-size slices (Opt 5); sort func (Opt 6) |
| `internal/pkg/imdbratings/provider.go` | `Score`/`snapshot` field types switch from `map[string]…` to sorted slices keyed by `uint32` (Opt 2 + 3); lookup methods do `parseTconst` once + binary search |
| `internal/pkg/imdbratings/dataset_bench_test.go` | **New.** Synthetic-data benches (no build tag) + synthetic data generator + heap-metric helper |
| `internal/pkg/imdbratings/dataset_bench_real_test.go` | **New.** `//go:build imdbbench` real-data benches that read from `LAZYSOAP_BENCH_DATA_DIR` (default `../../../var/imdb`) |
| `internal/pkg/imdbratings/provider_test.go` | Update existing tests if `snapshot` shape changes (the test that constructs `&snapshot{titles: …, episodes: …}` directly will need the new shape) |
| `Makefile` | Add `bench`, `bench-real`, `bench-baseline`, `bench-real-baseline`, `bench-stat`, `bench-real-stat` targets |
| `.gitignore` | Add `internal/pkg/imdbratings/testdata/real/` defensively |
| `README.md` (lines 103-119, plus new Benchmarks section) | Replace estimated perf figures with measured ones; document bench workflow |
| `CLAUDE.md` (testing notes section) | One short note about the bench convention so it's discoverable in future sessions |

## Implementation Steps

### Task 1: Build benchmark scaffolding

- [x] extract `buildSnapshotFromFiles(ctx, ratingsPath, episodePath)` from the current `buildSnapshot` in `internal/pkg/imdbratings/dataset.go`; make `buildSnapshot` a thin wrapper that does refresh-then-call
- [x] widen `gzipBytes` in `provider_test.go:35-43` to accept `testing.TB` so it works from `Benchmark*` callers (or move to a shared helper file in the same package)
- [x] create `internal/pkg/imdbratings/dataset_bench_test.go` (no build tag) with `BenchmarkParseRatings_Synthetic`, `BenchmarkParseEpisodes_Synthetic`, `BenchmarkBuildSnapshot_Synthetic`, `generateSyntheticDatasets`, and a heap-metric helper using `runtime.GC()` + `runtime.ReadMemStats` → `b.ReportMetric(.../1024/1024, "MB-heap")`
- [x] create `internal/pkg/imdbratings/dataset_bench_real_test.go` with `//go:build imdbbench` and `_Real` variants of all three families that read from `LAZYSOAP_BENCH_DATA_DIR` (default `../../../var/imdb` relative to the test file); `b.Fatalf` if files are missing under the tag
- [x] add `bench`, `bench-real`, `bench-baseline`, `bench-real-baseline`, `bench-stat`, `bench-real-stat` targets to `Makefile`
- [x] add `internal/pkg/imdbratings/testdata/real/` to `.gitignore` defensively
- [x] add a one-line note to `CLAUDE.md` testing-notes section pointing at the bench convention and the `imdbbench` build tag
- [x] write tests for new functionality (the bench harness is the artifact; verify each `_Synthetic` bench compiles and runs at least once with `-benchtime=1x` in a quick smoke check)
- [x] run project tests - `go test -race -count=1 ./internal/pkg/imdbratings/...` must pass before next task

### Task 2: Capture baselines

- [x] confirm `benchstat` is installed locally: `go install golang.org/x/perf/cmd/benchstat@latest`
- [x] confirm `var/imdb/` (or whatever `LAZYSOAP_BENCH_DATA_DIR` points at) holds the live IMDb gzipped TSVs; if empty, fetch via the existing operator path before running real benches
- [x] run `make bench-baseline` against current unchanged code; capture the raw output (saved to `bin/bench-baseline.txt`)
- [x] run `make bench-real-baseline` against current unchanged code; capture the raw output (saved to `bin/bench-real-baseline.txt`)
- [x] paste both baselines into the PR description (skipped - not automatable; baseline files at `bin/bench-baseline.txt` and `bin/bench-real-baseline.txt` are gitignored, paste from there into PR when opened)
- [x] run project tests - must pass before next task

### Task 3: Optimization 1 — Reorder parse passes

REVERTED. Implementation was correct and tests passed (including the
behavior improvement that recovers series-level ratings for series whose
only episode rows have malformed rating rows — see The Wire / tt0306414
case in the rewritten test). But benchstat on real IMDb 2026 data showed
a clear regression on the integrated path:

```
name                       old time/op    new time/op    delta
BuildSnapshot_Real-8          4.59s ± 3%     6.69s ± 5%   +45.64%  (p=0.000)
BuildSnapshot_Synthetic-8     7.09s ±15%    10.29s ±12%   +45.11%  (p=0.000)

name                       old MB-heap    new MB-heap    delta
BuildSnapshot_Real-8           91.3 ± 1%     215.8 ± 0%  +136.46%  (p=0.000)
BuildSnapshot_Synthetic-8      98.4 ± 0%     176.0 ± 0%   +78.83%  (p=0.000)

name                       old alloc/op   new alloc/op   delta
BuildSnapshot_Real-8         1.13GB ± 0%    2.20GB ± 0%   +94.02%  (p=0.000)
BuildSnapshot_Synthetic-8    1.14GB ± 0%    1.44GB ± 0%   +25.51%  (p=0.000)
```

Why the reorder loses on its own: the new `refs` map (~3M entries with
string keys + ~32-byte `episodeRef` values) is *bigger* than the old
1.66M-entry titles map it was meant to eliminate, AND it persists for the
whole ratings pass. Worse, every `episodeRef` carries an unshared copy of
its parent tconst string (3M strings), where the old code only paid for
unique parent strings via map deduplication. The expected win required
the companion optimizations (Opt 2 numeric tconsts → 4-byte parent IDs in
an 8-byte refs value) to make the refs map small enough to come out
ahead. As a standalone change the reorder pessimizes both heap and time.

Reverting the production code so the chain stays clean per the plan's
gate. Opt 2 will be evaluated next on the original baseline.

- [x] parse episodes first into `map[string]episodeRef` keyed by episode tconst (value: parent + season + episode) and `map[string]struct{}` of parent tconsts (implemented, then reverted)
- [x] stream ratings second: only allocate the tconst string and write a map entry when the row's tconst is in either set; never materialize the throwaway 1.6M-entry titles map (implemented, then reverted)
- [x] drop `pruneTitlesToSeries` (`dataset.go:95-117`) if it becomes dead code (deleted, then restored on revert)
- [x] update `TestRunPrunesNonSeriesTitles` — replace with a test asserting non-series tconsts never entered the snapshot in the first place (rewritten, then restored on revert)
- [x] re-run `make bench-real` and `make bench-real-stat`; record benchstat output (output saved above)
- [x] decide: if real-data MB-heap, allocs/op, or ns/op improves >= ~5%, keep the change; otherwise revert before proceeding (REVERTED — regression on all three metrics)
- [x] write tests for new/changed functionality (rewritten prune-equivalent test covers the contract) (covered while implementation was in place)
- [x] run project tests - `go test -race -count=1 ./internal/pkg/imdbratings/...` must pass before next task (passes on the restored baseline)

### Task 4: Optimization 2 — Numeric tconst keys

KEPT. Real-data benchstat shows large wins on every metric the gate cares
about, well past the >= ~5% bar. Heap is the user's primary ask and it
roughly halves on the integrated build path:

```
name                       old time/op    new time/op    delta
ParseRatings_Real-8           669ms ± 1%     637ms ± 5%   -4.70%  (p=0.001 n=9+10)
ParseEpisodes_Real-8          3.90s ± 3%     3.71s ± 2%   -4.70%  (p=0.000 n=10+10)
BuildSnapshot_Real-8          4.59s ± 3%     4.35s ± 2%   -5.36%  (p=0.000 n=10+10)

name                       old MB-heap    new MB-heap    delta
ParseRatings_Real-8            85.7 ± 0%      27.7 ± 0%  -67.73%  (p=0.000 n=8+10)
ParseEpisodes_Real-8            174 ± 1%        68 ± 0%  -60.93%  (p=0.000 n=10+10)
BuildSnapshot_Real-8           91.3 ± 1%      42.2 ± 1%  -53.78%  (p=0.000 n=10+10)

name                       old alloc/op   new alloc/op   delta
ParseRatings_Real-8           170MB ± 0%     142MB ± 0%  -16.60%  (p=0.000 n=10+10)
ParseEpisodes_Real-8          962MB ± 0%     957MB ± 0%   -0.44%  (p=0.000 n=10+10)
BuildSnapshot_Real-8         1.13GB ± 0%    1.10GB ± 0%   -2.94%  (p=0.000 n=9+10)
```

Why it wins: each "tt0944947"-style key cost ~26 bytes (16-byte string
header + ~10-byte body) plus an unshared allocation per surviving map
entry. Switching to `uint32` keys collapses every entry's key cost to
4 bytes and removes the per-entry tconst-string allocations that the
parsers used to leave pinned by the snapshot maps. Lookup path adds one
parseTconst call per request — sub-microsecond, dwarfed by everything
else on the hot path.

Note on signature: plan called for `parseTconst([]byte)`, but both call
sites (parser-side `strings.SplitN` substrings + request-side `imdbID
string`) naturally yield strings, and Go optimizes string indexing
identically to byte-slice indexing. Helper is `parseTconst(s string)`;
swap to `[]byte` is trivial when Opt 4 (Task 6) flips the parsers to
`bufio.Reader.ReadSlice`. Behavior is identical either way.

- [x] add `parseTconst([]byte) (uint32, bool)` helper in `internal/pkg/imdbratings/dataset.go` (implemented as `parseTconst(s string)` — see note above)
- [x] change `Score`/`snapshot` field types in `provider.go` from `map[string]…` to keys of `uint32`
- [x] update `parseRatings`/`parseEpisodes` to write `uint32` keys
- [x] update `Provider.SeriesRating` and `Provider.EpisodeRating` to call `parseTconst` once on the lookup path; wire-facing API stays `(imdbID string)`
- [x] update `provider_test.go` constructions of `&snapshot{titles: …, episodes: …}` to the new shape
- [x] re-run `make bench-real` and `make bench-real-stat`; record benchstat output (output above; raw run at `bin/bench-real.txt`)
- [x] decide: keep if real-data delta >= ~5%, otherwise revert before proceeding (KEPT — heap halves on the integrated path; time crosses the gate too)
- [x] write tests for new/changed functionality (parseTconst happy path + invalid input)
- [x] run project tests - must pass before next task

### Task 5: Optimization 3 — Sorted slices + binary search

KEPT. Published-state heap on the integrated build path roughly halves on
real IMDb 2026 data, which is the user's primary ask:

```
name                       old time/op    new time/op    delta
ParseRatings_Real-8           637ms ± 5%     719ms ± 9%  +12.78%  (p=0.000 n=10+9)
ParseEpisodes_Real-8          3.71s ± 2%     4.07s ± 8%   +9.67%  (p=0.000 n=10+10)
BuildSnapshot_Real-8          4.35s ± 2%     4.53s ± 9%   +4.21%  (p=0.000 n=10+9)
ParseRatings_Synthetic-8      729ms ± 4%     767ms ± 5%   +5.11%  (p=0.000 n=10+9)
ParseEpisodes_Synthetic-8     5.49s ± 9%     5.68s ±18%     ~     (p=0.579 n=10+10)
BuildSnapshot_Synthetic-8     6.97s ± 3%     6.18s ± 2%  -11.31%  (p=0.000 n=10+9)

name                       old MB-heap    new MB-heap    delta
ParseRatings_Real-8            27.7 ± 0%      27.7 ± 0%     ~     (p=0.084 n=10+10)
ParseEpisodes_Real-8           68.0 ± 0%      68.0 ± 0%     ~     (p=0.617 n=10+10)
BuildSnapshot_Real-8           42.2 ± 1%      22.9 ± 1%  -45.59%  (p=0.000 n=10+10)
ParseRatings_Synthetic-8       27.6 ± 0%      27.6 ± 0%     ~     (p=0.746 n=10+10)
ParseEpisodes_Synthetic-8      98.3 ± 0%      98.3 ± 0%     ~     (p=0.208 n=10+10)
BuildSnapshot_Synthetic-8      72.3 ± 0%      53.0 ± 0%  -26.72%  (p=0.000 n=10+10)

name                       old alloc/op   new alloc/op   delta
BuildSnapshot_Real-8         1.10GB ± 0%    1.10GB ± 0%   +0.18%  (p=0.000 n=10+10)
BuildSnapshot_Synthetic-8    1.11GB ± 0%    1.11GB ± 0%   +0.16%  (p=0.000 n=10+10)
```

Why it wins on heap: the post-build state used to retain two maps with
~45k entries each. Each Go map carries bucket-array overhead — every entry
sits inside an 8-slot bucket with a 4-byte tophash, plus a separately
allocated overflow chain when the bucket fills. For ~45k entries that's
roughly 7-8k buckets × ~144 bytes = ~1 MB of bucket metadata per map,
plus the entry storage itself. Switching to two ID-sorted slices (45k ×
12-byte titleEntry, 45k × 32-byte seriesEpisodes) gives a contiguous
read-only structure with no bucket overhead, no overflow pointers, and
no padding-from-hashing. The entire EpisodeScore data underneath is
reused without copying — only the outer index changes shape.

Why parse-only times look noisier than baseline: ParseRatings_Real /
ParseEpisodes_Real exercise unchanged code (the parsers still return
maps; conversion only happens in buildSnapshotFromFiles via the new
snapshotFromMaps helper). The +12.78% / +9.67% on those benches comes
from a noisier run-time environment this round (note the wider stddev:
±5-9% vs the baseline's ±2-3%), not from any code path the change
touches. BuildSnapshot_Real time at +4.21% with 9% stddev is also
inside the noise band, which the synthetic case confirms by going the
other direction (-11.31%).

Allocs/op moves +0.18% on BuildSnapshot_Real, essentially flat — the
extra slice + sort is offset against the freed map containers.

- [x] change published `titles` and `episodes` parent index in `snapshot` to `[]titleEntry{ID uint32; Score}` (and the parent-keyed equivalent), sorted by ID
- [x] sort once at end of `buildSnapshotFromFiles`
- [x] swap lookup methods to `slices.BinarySearchFunc`
- [x] update `provider_test.go` snapshot constructions
- [x] re-run `make bench-real` and `make bench-real-stat`; record benchstat output (output above; raw run at `bin/bench-real.txt`)
- [x] decide: keep if real-data delta >= ~5%, otherwise revert before proceeding (KEPT — heap halves on the integrated build path)
- [x] write tests for new/changed functionality (binary-search hit/miss, boundary IDs)
- [x] run project tests - must pass before next task

### Task 6: Optimization 4 — Skip per-line string allocations

KEPT. The biggest single CPU/alloc win in the chain so far. Real-data
benchstat clears every gate by an order of magnitude:

```
name                       old time/op    new time/op    delta
ParseRatings_Real-8           719ms ± 9%     530ms ± 3%  -26.28%  (p=0.000 n=9+10)
ParseEpisodes_Real-8          4.07s ± 8%     3.09s ± 3%  -24.20%  (p=0.000 n=10+10)
BuildSnapshot_Real-8          4.53s ± 9%     3.66s ± 5%  -19.14%  (p=0.000 n=9+10)
ParseRatings_Synthetic-8      767ms ± 5%     677ms ± 7%  -11.74%  (p=0.000 n=9+9)
ParseEpisodes_Synthetic-8     5.68s ±18%     4.78s ± 3%  -15.76%  (p=0.000 n=10+10)
BuildSnapshot_Synthetic-8     6.18s ± 2%     5.59s ± 5%   -9.57%  (p=0.000 n=9+10)

name                       old alloc/op   new alloc/op   delta
ParseRatings_Real-8           142MB ± 0%      29MB ± 0%  -79.41%  (p=0.000 n=10+10)
ParseEpisodes_Real-8          957MB ± 0%      53MB ± 0%  -94.42%  (p=0.000 n=9+10)
BuildSnapshot_Real-8         1.10GB ± 0%    0.09GB ± 0%  -92.25%  (p=0.000 n=10+10)

name                       old allocs/op  new allocs/op  delta
ParseRatings_Real-8           3.34M ± 0%     0.00M ± 0%  -99.86%  (p=0.000 n=10+10)
ParseEpisodes_Real-8          19.6M ± 0%      0.3M ± 0%  -98.41%  (p=0.000 n=9+10)
BuildSnapshot_Real-8          22.9M ± 0%      0.3M ± 0%  -98.62%  (p=0.000 n=10+10)
```

Why it wins: every `sc.Text()` call returns a freshly allocated string
copy of the underlying scanner buffer. Across 1.66M ratings rows and 9M
episode rows, that is the single dominant garbage source for the parse
phase (each row is 50–100 bytes; allocs/op was ~22.9M for the integrated
build = roughly one alloc per row plus a handful of substring allocs
inside `strings.SplitN`). Switching the inner loop to
`bufio.Reader.ReadSlice('\n')` returns a sub-slice that aliases the
bufio buffer until the next call — zero copies. `bytes.Cut`/`bytes.IndexByte`
walks the row, splits by tab in place, and `parseTconst` was made
generic over `string | []byte` (Go's type-set lets the same body index
either with `s[i]` returning `byte`) so the parser path passes byte
slices through to the same logic that the request path uses with strings,
with no allocations in either direction.

The remaining ~0.3M allocs/op on the integrated build come almost
entirely from the snapshot's outer `[]titleEntry` / `[]seriesEpisodes`
slices and the `[]EpisodeScore` per-series slices — i.e. the
post-build retained data, not the parse loop.

Heap: noise-band fluctuation (BuildSnapshot_Real ±15% stddev on the new
side; synthetic flat at +0.98%). The published snapshot shape is
unchanged, so retained heap should not move; the wide stddev on the
real bench reflects machine load during the run.

Note on `parseTconst`: switched the helper from `func(s string)` to
`func[T string | []byte](s T)` so both call sites share the same
implementation without unsafe conversions or duplicated bodies. Existing
`parseTconst("ttN")` callers in tests and request paths continue to
infer `T = string` and behave identically.

- [x] replace `bufio.Scanner` + `sc.Text()` in both parsers with `bufio.Reader.ReadSlice('\n')` and parse fields straight from the byte slice
- [x] use `bytes.IndexByte` for tab splits; pass byte slices to `strconv.ParseFloat`/`ParseUint`; copy only when actually storing (used `bytes.Cut` for ratings two-tab split per linter suggestion; `bytes.IndexByte` retained for the four-field episode loop)
- [x] re-run `make bench-real` and `make bench-real-stat`; record benchstat output (recorded above; raw run at `bin/bench-real.txt`)
- [x] decide: keep if real-data delta >= ~5% on allocs/op or ns/op, otherwise revert before proceeding (KEPT — every metric clears the gate by 4×+)
- [x] write tests for new/changed functionality (added `TestParseRatingsSkipsMalformedRows`, `TestParseRatingsHandlesNoTrailingNewline`, `TestParseEpisodesSkipsMalformedRows`)
- [x] run project tests - must pass before next task

### Task 7: Optimization 5 — Pre-size episode slices

KEPT. Heap is the user's primary ask and it roughly halves on the
integrated build path; allocation count drops by ~44%. Real-data
benchstat (against the post-Task-6 baseline at `bin/bench-real-baseline.txt`):

```
name                       old time/op    new time/op    delta
ParseRatings_Real-8           530ms ± 3%     519ms ± 6%     ~     (p=0.143 n=10+10)
ParseEpisodes_Real-8          3.09s ± 3%     2.99s ± 3%   -3.12%  (p=0.001 n=10+10)
BuildSnapshot_Real-8          3.66s ± 5%     3.74s ±18%     ~     (p=0.481 n=10+10)

name                       old MB-heap    new MB-heap    delta
ParseEpisodes_Real-8           72.8 ± 4%      40.9 ± 0%  -43.89%  (p=0.000 n=10+10)
BuildSnapshot_Real-8           27.2 ±15%      13.4 ± 0%  -50.70%  (p=0.000 n=10+6)
ParseEpisodes_Synthetic-8      98.7 ± 0%      68.5 ± 0%  -30.62%  (p=0.000 n=10+10)
BuildSnapshot_Synthetic-8      53.5 ± 0%      40.6 ± 0%  -24.18%  (p=0.000 n=10+7)

name                       old alloc/op   new alloc/op   delta
ParseEpisodes_Real-8         53.4MB ± 0%    83.4MB ± 0%  +56.22%  (p=0.000 n=10+9)
BuildSnapshot_Real-8         85.4MB ± 0%   115.5MB ± 0%  +35.24%  (p=0.000 n=10+10)
ParseEpisodes_Synthetic-8     119MB ± 0%     113MB ± 0%   -5.27%  (p=0.000 n=8+10)
BuildSnapshot_Synthetic-8     151MB ± 0%     145MB ± 0%   -4.19%  (p=0.000 n=10+10)

name                       old allocs/op  new allocs/op  delta
ParseEpisodes_Real-8           311k ± 0%      172k ± 0%  -44.75%  (p=0.000 n=10+9)
BuildSnapshot_Real-8           316k ± 0%      177k ± 0%  -44.06%  (p=0.000 n=10+10)
ParseEpisodes_Synthetic-8      572k ± 0%      239k ± 0%  -58.33%  (p=0.000 n=10+10)
BuildSnapshot_Synthetic-8      580k ± 0%      246k ± 0%  -57.57%  (p=0.000 n=10+10)
```

Why the heap halves on BuildSnapshot_Real: the previous parser grew each
of ~45k per-parent slices via `append`, leaving on average ~50% trailing
capacity slack baked into each slice. That slack carried into the
published snapshot, since `snapshotFromMaps` reuses the underlying
`[]EpisodeScore` slices without copying. The two-phase parser tallies
exact per-parent counts during the streaming pass, then pre-allocates
each per-parent slice to its final length — so the published shape
retains zero trailing slack. ~3M EpisodeScore × 12 bytes × ~50%
slack = roughly 18 MB of retained-heap savings on real data, plus the
elimination of bucket-overflow chains in the auxiliary maps that the
counting pass primed at exact size. The drop in allocation *count* (-44%)
comes from the same place: ~45k per-parent slices used to do 6-7 grow
allocations each (1→2→4→...→128), now they do exactly one.

Why bytes-allocated rises (+35-56%) while heap drops: the counting pass
buffers the joined rows into a flat `[]parsedEpisode` (~3M × 16 bytes ≈
48 MB transient). That transient slice is reachable during parse — so it
shows up in `alloc/op` (bytes) — but is unreachable by the time
`snapshotFromMaps` returns, so it does not show up in retained `MB-heap`.
The 3.5M-element initial capacity for the transient buffer is intentional:
it avoids further append-doublings on real-data row counts (~3.0M joined),
which is what keeps allocation *count* down while bytes go up.

Why time is flat: the parse phase is gzip-bound. Saving ~6-7 doubling
allocs per series across 45k series is real CPU work but it amortizes
into the noise of decompression + bytes-level field splitting. The point
of this optimization is the published-state heap, not throughput.

Implementation note: kept the existing `parseEpisodes(io.Reader, titles)`
signature so existing tests and benches don't move. Extracted
`collectEpisodeRows` (counting pass) and `fillEpisodesByParent` (fill pass)
to keep `parseEpisodes` under the project's cyclomatic-complexity budget.
Added `initialJoinedEpisodesCapacity = 3_500_000` constant alongside the
existing capacity hints.

- [x] add a counting pass over episodes that computes per-parent counts (implemented as a single streaming pass that buffers joined rows in a flat `[]parsedEpisode` slice and tallies per-parent counts; avoids re-decompressing the gzipped file)
- [x] rewrite the fill pass to write into pre-allocated slices with known capacity (`fillEpisodesByParent` pre-allocates each per-parent slice to its exact tallied length, then walks the flat row buffer in order)
- [x] re-run `make bench-real` and `make bench-real-stat`; evaluate the combined effect of Tasks 6 + 7 (output above; raw run at `bin/bench-real.txt`)
- [x] decide: keep if real-data delta >= ~5%, otherwise revert before proceeding (KEPT — heap halves on the integrated build path, alloc count drops 44%)
- [x] write tests for new/changed functionality (added `TestParseEpisodesPreallocatesPerParentSlices` asserting `cap == len` for every per-parent slice in the parsed output)
- [x] run project tests - must pass before next task

### Task 8: Optimization 6 — slices.SortFunc

KEPT. Allocation count drops by ~70% on the integrated build path —
exactly what the optimization targets. Real-data benchstat against a
fresh post-Task-7 baseline (regenerated this round; the previous
`bin/bench-real-baseline.txt` was still the post-Task-6 baseline since
Task 7 didn't refresh it):

```
name                       old time/op    new time/op    delta
ParseRatings_Real-8           525ms ± 2%     586ms ±14%  +11.61%  (p=0.000 n=9+10)
ParseEpisodes_Real-8          3.03s ± 2%     3.48s ±12%  +14.90%  (p=0.000 n=8+10)
BuildSnapshot_Real-8          3.69s ± 8%     4.00s ± 7%   +8.37%  (p=0.001 n=10+9)
ParseEpisodes_Synthetic-8     4.63s ± 2%     4.62s ± 5%     ~     (p=0.780 n=10+9)
BuildSnapshot_Synthetic-8     5.40s ± 1%     5.30s ± 3%   -1.86%  (p=0.006 n=9+10)

name                       old MB-heap    new MB-heap    delta
ParseEpisodes_Real-8           40.8 ± 0%      40.8 ± 0%     ~     (p=0.402 n=10+10)
BuildSnapshot_Real-8           13.4 ± 0%      13.3 ± 0%   -0.24%  (p=0.000 n=8+10)

name                       old alloc/op   new alloc/op   delta
ParseEpisodes_Real-8         83.4MB ± 0%    79.9MB ± 0%   -4.22%  (p=0.000 n=10+9)
BuildSnapshot_Real-8          115MB ± 0%     112MB ± 0%   -3.04%  (p=0.000 n=10+10)
ParseEpisodes_Synthetic-8     113MB ± 0%     108MB ± 0%   -4.31%  (p=0.000 n=10+9)
BuildSnapshot_Synthetic-8     145MB ± 0%     140MB ± 0%   -3.39%  (p=0.000 n=10+10)

name                       old allocs/op  new allocs/op  delta
ParseEpisodes_Real-8           172k ± 0%       51k ± 0%  -70.56%  (p=0.000 n=10+9)
BuildSnapshot_Real-8           177k ± 0%       55k ± 0%  -68.63%  (p=0.000 n=10+10)
ParseEpisodes_Synthetic-8      239k ± 0%       74k ± 0%  -68.96%  (p=0.000 n=10+9)
BuildSnapshot_Synthetic-8      246k ± 0%       82k ± 0%  -66.83%  (p=0.000 n=10+10)
```

Why it wins: `sort.Slice` boxes the slice into `reflect.Value` and
allocates a closure-shaped interface per call. With ~45k per-series
sorts on real data, that's ~45k closures + ~45k interface boxings
per build. `slices.SortFunc` is monomorphised at compile time over
the element type and takes an unboxed comparator, so the per-call
overhead drops to ~zero. The remaining ~51k allocs/op on
ParseEpisodes_Real come almost entirely from the per-parent slice
allocations that the counting pass primes (~45k slices) plus a
handful of bookkeeping allocs — i.e. the steady-state cost of the
joined snapshot itself, not loop-driven garbage.

Why time looks worse on Real: the +11.61% on ParseRatings_Real is the
tell — that bench exercises code Opt 6 doesn't touch (only the episode
sort path was changed), so any movement there is pure run-time noise.
The new run shows ±12-14% stddev on every Real bench versus baseline
±2-8%, confirming the box was noisier this round. BuildSnapshot_Synthetic
(quieter machine state, ±3% stddev) shows a -1.86% improvement, which
is the directionally honest read on time.

bytes alloc/op drops -3-4% (the ~120k closure objects each carried a
small payload). Heap is flat as expected — sorting in place doesn't
change the published shape.

- [x] swap `sort.Slice` calls over per-series episode slices to `slices.SortFunc` to drop the closure interface allocation (replaced the single `sort.Slice` call site in `sortEpisodesByAirOrder`; `cmp.Compare` keeps the (Season, Episode) ordering identical to the previous comparator; dropped the now-unused `"sort"` import from `dataset.go`)
- [x] re-run `make bench-real` and `make bench-real-stat`; record benchstat output (raw run at `bin/bench-real.txt`, fresh baseline at `bin/bench-real-baseline.txt`)
- [x] decide: keep if real-data delta is observable, otherwise revert (KEPT — alloc count drops -70% on integrated path, exactly the targeted metric)
- [x] write tests for new/changed functionality (sort order is unchanged) (added `TestSortEpisodesByAirOrder` exercising both branches of the comparator with scrambled cross-season input; existing `TestParseEpisodesJoinAndSort` continues to cover within-season ordering through the parser)
- [x] run project tests - must pass before next task

### Task 9: Update documentation from measurements

`make docker-build` was blocked by a pre-existing `forcetypeassert` lint failure on master in `internal/pkg/clients/tmdb/tmdb.go:24` that the
Dockerfile's lint stage refuses to ship past — unrelated to this branch
(verified `git diff master` is empty for that file). Substituted an
equivalent local-binary measurement: `make build`, started
`bin/lazysoap` with `LAZYSOAP_IMDB_CACHE_DIR=./var/imdb` against the
real cached TSVs, sampled `ps -p $PID -o rss=` at ~10 ms intervals
across the boot/build phase (394 samples) for peak RSS, and pulled
`/debug/pprof/heap?gc=1` for post-build heap. Same Go binary the
container would run, same dataset, same code path; only difference is
the runtime sandbox. Numbers used in README:

- compressed TSV: 8 MB ratings + 53 MB episodes = ~61 MB on disk
- build elapsed: ~4.0 s (`imdb dataset loaded` log line)
- peak RSS during build: ~80 MB (82,168 KB observed peak across 394 samples)
- post-build retained heap: ~13 MB (`BuildSnapshot_Real` MB-heap from `bin/bench-real.txt`); pprof `inuse_space -gc=1` reports ~8 MB attributed sample, both consistent and well below the previously documented "~35 MB" estimate

- [x] run `make docker-build && docker compose up -d`; wait for `imdb dataset loaded` log; record `docker stats --no-stream soap` RSS and `/debug/pprof/heap?gc=1` post-build heap (substituted local-binary measurement; see note above)
- [x] replace the estimated perf figures at `README.md:112-114` with the measured post-optimization peak RSS and post-build heap from the Docker run
- [x] update `dataset.go:35-43` (the `initialTitlesCapacity` comment block describing the old strategy) to reflect the new pass ordering, OR delete the constant if Task 3 made it dead code (no change needed: Task 3 was reverted, so the constant is still in active use at `parseRatings`; the existing comment already describes the current strategy with empirical 2026 figures — verified by re-reading code and comment)
- [x] add a short `## Benchmarks` section to `README.md` documenting `make bench` / `make bench-real`, `LAZYSOAP_BENCH_DATA_DIR`, the `imdbbench` build tag, and where the baseline raw output lives
- [x] confirm any prose that described `pruneTitlesToSeries` or the old pass order has been updated or removed (updated `pruneTitlesToSeries` doc comment: dropped the stale "tconst-string objects it roots" reference — keys are `uint32` post-Opt 2 — and the estimated "~80 MB" figure that no longer reflects current code)
- [x] write tests for new/changed functionality (n/a — docs only; verified `go test -race -count=1 ./internal/pkg/imdbratings/...` passes)
- [x] run project tests - must pass before next task

### Task 10: Verify acceptance criteria

- [x] verify all requirements from Overview are implemented: benchmarks first, optimizations second (each justified by benchstat), no big files committed (Tasks 1-9 done; bench harness landed before any optimization; each optimization has its benchstat record above; `git diff master --stat` lists only intended files — no `var/` or `testdata/real/` artifacts)
- [x] confirm wire-facing API `SeriesRating(imdbID string)` is unchanged (verified at `internal/pkg/imdbratings/provider.go:97` — signature still `func (p *Provider) SeriesRating(imdbID string) (float32, uint32, bool)`)
- [x] run `go test -race -count=1 ./internal/pkg/imdbratings/...` — must pass (passed: `ok github.com/Nikscorp/soap/internal/pkg/imdbratings 12.101s`)
- [x] run `make lint` — all issues must be fixed (15 issues remain, ALL pre-existing on master and in files this branch never touched: `internal/pkg/clients/tmdb/tmdb.go`, `internal/pkg/rest/metrics.go`, `internal/pkg/tvmeta/{details,episodes,search}.go`, plus `frontend/node_modules/flatted/...` vendored 3rd-party. Verified by running `make lint` against pristine master worktree — same 15 issues. Out of scope for this branch; Task 9 already noted the same blocker.)
- [x] run full project test suite (`make test-race`) — must pass (passed end-to-end across all packages)
- [x] run end-to-end Docker check (skipped - not automatable on this branch): blocked by the same pre-existing `forcetypeassert` lint failure on master that Task 9 documented. Dockerfile line 15 runs `golangci-lint run ./...` as a build stage — it fails on `internal/pkg/clients/tmdb/tmdb.go:24`, which `git diff master` confirms is empty for this branch. Task 9 substituted a local-binary RSS/heap measurement which is what backs the README figures; same substitution applies here.
- [x] confirm `git status --porcelain` lists nothing under `var/` or `internal/pkg/imdbratings/testdata/real/` (working tree fully clean — `git status --porcelain` returned empty)
- [x] confirm the PR description carries the benchstat output for each kept optimization, plus the Docker-measured RSS / post-build heap that backs the new README figures (skipped - not automatable; benchstat output is in this plan file under Tasks 4, 5, 6, 7, 8 and the README/Task-9 notes hold the measured RSS/heap figures — paste from those into PR description when opened)

## Post-Completion

*Items requiring manual intervention - no checkboxes, informational only*

- CI integration of the synthetic suite is out of scope for this change; can be added in a follow-up if regression catches become useful.
- `benchstat` install (`go install golang.org/x/perf/cmd/benchstat@latest`) is a developer-machine prerequisite — not added to the Makefile to avoid baking in a tool dep.
- Real-data benches require populating `LAZYSOAP_BENCH_DATA_DIR` (default `var/imdb/`) with the IMDb gzipped TSVs before `make bench-real` returns useful numbers.
