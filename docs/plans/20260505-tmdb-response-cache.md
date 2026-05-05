# TMDB response cache (tvmeta layer)

## Overview

Add an in-memory, TTL-bounded LRU cache for parsed TMDB responses inside the `internal/pkg/tvmeta` layer. Today every `/id/{id}`, `/search/{q}`, and `/featured` request fans out fresh TMDB calls — `/id/{id}` in particular issues 1 + N (one per season) HTTP round trips against TMDB on every hit, even when the same series was just requested. Latency on `/id/{id}` is dominated by TMDB; popular shows are repeatedly re-fetched.

The cache stores already-parsed domain structs (`*TvShowDetails`, `*TVShowSeasonEpisodes`, `*TVShows`) keyed by the inputs that vary the response (id/season/query + language). It uses a typed LRU with per-method TTL and `singleflight` to dedupe concurrent identical fetches.

**Goals (per planning conversation):**
1. Cut `/id/{id}` latency on warm hits (the heavy multi-call endpoint).
2. Reduce TMDB request volume globally (stay further below the 50 req/s cap, save bandwidth).

**Out of scope:**
- `PopularTVShows` is intentionally not cached: `/featured` shuffles per request and the popular pool changes daily; an extra cache layer here is marginal next to the existing `featuredExtras` cache.
- `GetTVExternalIDs` is already cached unbounded in `Client.imdbIDCache` — leave it alone.
- No persistent / on-disk cache (in-memory only).
- No external Redis / shared cache (single-instance Docker deployment).

## Context (from discovery)

**Files involved:**
- `internal/pkg/tvmeta/client.go` — `Client` struct, `tmdbClient` interface, existing `imdbIDCache`. The construction seam (`New`) is where the cache config is plumbed in.
- `internal/pkg/tvmeta/details.go` — `TVShowDetails` (cache target).
- `internal/pkg/tvmeta/episodes.go` — `TVShowEpisodesBySeason` (cache target; biggest latency win because `TVShowAllSeasonsWithDetails` fans this out N times per `/id/{id}`).
- `internal/pkg/tvmeta/search.go` — `SearchTVShows` (cache target; ratings override happens *inside* the method and mutates results — needs split into raw + post-process).
- `internal/pkg/tvmeta/all_seasons_with_details.go` — orchestrates per-season fan-out + ratings override; transparently benefits from cached underlying methods.
- `cmd/lazysoap/main.go` — wiring; passes config into `tvmeta.New`.
- `internal/pkg/config/config.go` — top-level `Config` tree; new `tvmeta.CacheConfig` block needs to be embedded.
- `config/config.yaml.dist` — runtime defaults reference.
- `internal/pkg/tvmeta/mocks/` — minimock-generated mocks; `tmdbClient` mock already exists, no signature changes expected.
- `README.md` + `CLAUDE.md` — architecture section needs the new cache documented alongside the existing `imdbIDCache` and `featuredExtrasCache` notes.

**Related patterns found:**
- Existing in-process caches: `Client.imdbIDCache` (sync.Map, unbounded — appropriate because the value space is bounded ~200k strings); `featuredExtrasCache` (atomic.Pointer copy-on-write with background refresh). The new TMDB response cache fits between these in shape: bounded LRU + TTL.
- Concurrent fan-out cap: `externalIDsConcurrency = 8` and `featuredExtraIDsConcurrency = 4` are precedents for explicit concurrency budgets aimed at TMDB's 50 req/s ceiling. The new cache should mention this in comments — singleflight dedupes thundering herds at the cache layer, not the rate limit layer.
- Tests: minimock-driven, table-driven `_test.go` next to source. Lint disables tests (`tests: false` in `.golangci.yml`).

**Dependencies to add:**
- `github.com/hashicorp/golang-lru/v2` (provides `expirable.LRU[K, V]`) — mature, generic, no dependency surface.
- `golang.org/x/sync/singleflight` — already partially used (`golang.org/x/sync/errgroup` is vendored), adds one sub-package.

**Non-obvious wrinkles:**
- `SearchTVShows` mutates results with `overrideSeriesRatings` *inside* the method. Caching the post-override slice is unsafe: the IMDb dataset refresh interval is 24h, so a 6h-cached search result could carry ratings older than a fresh provider snapshot. Fix by splitting into a private `searchTVShowsRaw` (cached, no override) + a public `SearchTVShows` (calls raw, then applies override on the returned shared pointer's deep copy — overrides mutate, so a cached pointer must not be the source of truth for `Rating`). Plan handles this in the SearchTVShows task.
- Cached values are pointers (`*TvShowDetails`, etc.). Concurrent readers share them; like the existing `featuredExtras.view()` slice, they must be treated as read-only by callers. Document this on the cache API and on each cached method.
- `singleflight` delivers the same result to all waiters even if one waiter's context was cancelled. The fetch fn must use a non-cancellable derived context (`context.WithoutCancel`-style or `context.Background()` with a fresh timeout), so a single cancelled caller doesn't poison cache for everyone else. Standard pattern; tested explicitly.

## Development Approach
- **Testing approach**: Regular (code first, then tests).
- Complete each task fully before moving to the next.
- Make small, focused changes.
- **CRITICAL: every task MUST include new/updated tests** for code changes in that task.
  - tests are not optional - they are a required part of the checklist
  - write unit tests for new functions/methods
  - write unit tests for modified functions/methods
  - add new test cases for new code paths
  - update existing test cases if behavior changes
  - tests cover both success and error scenarios
- **CRITICAL: all tests must pass before starting next task** - no exceptions.
- **CRITICAL: update this plan file when scope changes during implementation.**
- Run `make test-race` after each task that touches concurrent code (cache primitive, singleflight integration, all the wrappers). The cache is on the request hot path — race detector hits here are the failure mode that survives unit tests.
- Maintain backward compatibility: cache is opt-in by config, defaults must be safe (small TTL, modest sizes); zero-config builds keep working.

## Testing Strategy
- **Unit tests**: required for every task (see Development Approach above). Use the existing minimock `tmdbClient` mock to verify cache hits avoid downstream calls (mock expectation: called exactly N times, not 2N).
- **E2E tests**: `frontend/e2e/` is a single Playwright smoke test — only re-run if frontend changes. This change is backend-only, so e2e is just the existing smoke (must still pass against the rebuilt Docker image).
- **Race detection**: `make test-race` for `internal/pkg/tvmeta/...` after every task that adds concurrent code.
- **Latency measurement** (verification task): cold-vs-warm `time curl -fsS http://127.0.0.1:8202/id/42009` against the running container. Required by CLAUDE.md docs discipline — README cache claims must come from a real number, not "should be faster".

## Progress Tracking
- Mark completed items with `[x]` immediately when done.
- Add newly discovered tasks with ➕ prefix.
- Document issues/blockers with ⚠️ prefix.
- Update plan if implementation deviates from original scope.
- Keep plan in sync with actual work done.

## What Goes Where
- **Implementation Steps** (`[ ]` checkboxes): code, tests, docs that the agent can complete.
- **Post-Completion** (no checkboxes): manual measurement, deployment notes, future work.

## Implementation Steps

### Task 1: Add cache + singleflight dependencies
- [x] `go get github.com/hashicorp/golang-lru/v2@latest`
- [x] `go get golang.org/x/sync@latest` (for `singleflight`; already used for `errgroup`)
- [x] run `make tidy` to update `go.sum` and re-vendor under `vendor/`
- [x] verify `make build` and `make lint` still pass with new deps vendored
- [x] confirm vendored packages exist: `vendor/github.com/hashicorp/golang-lru/v2/expirable/` and `vendor/golang.org/x/sync/singleflight/`
- [x] run `make test` — must pass before next task

### Task 2: Generic `responseCache[K, V]` primitive
- [x] create `internal/pkg/tvmeta/cache.go` with a typed `responseCache[K comparable, V any]` struct wrapping `expirable.LRU[K, V]` + `singleflight.Group`
- [x] implement `(*responseCache).GetOrFetch(ctx, key, fetch func(context.Context) (V, error)) (V, error)`:
  - on hit: return cached value, no fetch call
  - on miss: dedupe via `singleflight.DoChan` (key = stringified cache key); fetch fn receives a context detached from caller cancellation (`context.WithoutCancel(ctx)`) so a cancelled waiter doesn't kill the fetch for other waiters
  - errors: do NOT cache (transient TMDB failures must not poison the cache); return error to all waiters of the same singleflight slot
- [x] expose `(*responseCache).len() int` and a constructor `newResponseCache[K, V](size int, ttl time.Duration) *responseCache[K, V]` (size <= 0 or ttl <= 0 returns a no-op pass-through cache so config can disable per-method caching)
- [x] write tests in `internal/pkg/tvmeta/cache_test.go`:
  - hit returns cached value, fetch fn not called second time (use a counter)
  - miss after TTL re-fetches (use synthetic clock or short TTL with `time.Sleep` — prefer real time, ttl=50ms)
  - error from fetch is returned, not cached (next call re-tries fetch)
  - concurrent N callers with same key trigger fetch fn exactly once (singleflight verification)
  - cancelled caller context: result still delivered if fetch succeeds before cancellation; cancelled caller observes its own ctx error otherwise (chosen semantics: caller-ctx cancellation cancels the *wait*, not the fetch; other waiters still get the value)
  - disabled cache (size=0 or ttl=0) calls fetch fn every time
- [x] run `make test-race ./internal/pkg/tvmeta/...` — must pass before next task

### Task 3: Cache `TVShowDetails`
- [x] add `detailsCache *responseCache[detailsKey, *TvShowDetails]` field to `Client` (where `detailsKey` is a comparable struct `{ id int; lang string }`)
- [x] initialize from `Config.Cache.Details{Size,TTL}` in `New`
- [x] in `TVShowDetails`: build the key, call `detailsCache.GetOrFetch`; the fetch fn does the existing `c.client.GetTVDetails` + parse path
- [x] document on the method that the returned `*TvShowDetails` is shared and read-only (callers must not mutate)
- [x] update tests in `internal/pkg/tvmeta/details_test.go`:
  - existing happy / error tests still pass with cache enabled
  - new test: two calls with same (id, lang) hit TMDB exactly once (assert mock `GetTVDetails` called once via minimock counter)
  - new test: different `lang` keys do NOT collide
  - new test: error from TMDB is returned and is not cached (mock expects 2 calls for 2 errored requests)
- [x] run `make test-race ./internal/pkg/tvmeta/...` — must pass before next task

### Task 4: Cache `TVShowEpisodesBySeason` (the biggest win)
- [x] add `episodesCache *responseCache[episodesKey, *TVShowSeasonEpisodes]` field to `Client` (`episodesKey` = `{ id, season int; lang string }`)
- [x] initialize from `Config.Cache.Episodes{Size,TTL}` in `New`
- [x] in `TVShowEpisodesBySeason`: build key, call `episodesCache.GetOrFetch`; fetch fn = existing `c.client.GetTVSeasonDetails` + parse loop
- [x] document shared-read-only contract on the returned `*TVShowSeasonEpisodes`. Important: `TVShowAllSeasonsWithDetails` later applies `overrideEpisodeRatings` which **mutates `ep.Rating`**. Cached pointer must therefore be deep-copied before mutation, OR overrides must be applied to a per-call copy of the episodes slice. Choose: deep-copy in `TVShowAllSeasonsWithDetails` after gathering seasons, before override (1 small alloc per season; clean separation).
- [x] update `TVShowAllSeasonsWithDetails` to deep-copy episodes from each cached season pointer before passing them to `overrideEpisodeRatings`. Add a brief comment explaining why (cached value, override mutates).
- [x] update tests in `internal/pkg/tvmeta/episodes_test.go` and `all_seasons_with_detail_test.go`:
  - same-key cache hit verified
  - cached episodes are isolated from override mutation (call `TVShowAllSeasonsWithDetails` twice with a ratings provider returning different ratings — cached side must not carry first call's overrides)
  - different (id, season, lang) tuples do not collide
  - error not cached
- [x] run `make test-race ./internal/pkg/tvmeta/...` — must pass before next task

### Task 5: Cache `SearchTVShows` (split raw + override)
- [x] extract a private `(c *Client) searchTVShowsRaw(ctx context.Context, query string) (*TVShows, error)` containing the current TMDB call + parse + popularity sort, **without** calling `overrideSeriesRatings`
- [x] make `SearchTVShows` call `searchTVShowsRaw` via a `searchCache *responseCache[searchKey, *TVShows]` (`searchKey` = `{ query, lang string }` — note: `languageTag(query)` is currently derived from the query, so the lang component is the resolved tag, not the input)
- [x] after fetching from cache (or computing), produce a per-call deep copy of the `[]*TVShow` slice (and pointed-to structs) before calling `overrideSeriesRatings` on it; assemble a fresh `*TVShows` to return. This preserves the rule that cached pointers stay read-only AND keeps callers seeing the freshest IMDb-overridden ratings on every call.
- [x] alternative considered: snapshot the ratings provider's "ready / dataset version" into the cache key — rejected because the IMDb provider doesn't expose a version stamp today, and adding one for this is scope creep.
- [x] update tests in `internal/pkg/tvmeta/search_test.go`:
  - same query + ratings-provider state: TMDB called once, override called twice (override is per-call by design)
  - simulated ratings change between two calls: second response reflects new ratings even though TMDB was hit only once (asserts override-runs-on-cached-data semantics)
  - error not cached
  - different query strings do not collide
- [x] run `make test-race ./internal/pkg/tvmeta/...` — must pass before next task

### Task 6: `tvmeta.CacheConfig` plumbing
- [ ] add `CacheConfig` struct to `internal/pkg/tvmeta/cache.go` (or a new `config.go`) with fields per cached method:
  ```go
  type CacheConfig struct {
      DetailsSize     int           `env:"TVMETA_CACHE_DETAILS_SIZE"      env-default:"1024" yaml:"details_size"`
      DetailsTTL      time.Duration `env:"TVMETA_CACHE_DETAILS_TTL"       env-default:"6h"   yaml:"details_ttl"`
      EpisodesSize    int           `env:"TVMETA_CACHE_EPISODES_SIZE"     env-default:"4096" yaml:"episodes_size"`
      EpisodesTTL     time.Duration `env:"TVMETA_CACHE_EPISODES_TTL"      env-default:"6h"   yaml:"episodes_ttl"`
      SearchSize      int           `env:"TVMETA_CACHE_SEARCH_SIZE"       env-default:"256"  yaml:"search_size"`
      SearchTTL       time.Duration `env:"TVMETA_CACHE_SEARCH_TTL"        env-default:"30m"  yaml:"search_ttl"`
  }
  ```
- [ ] change `tvmeta.New` signature to `New(client tmdbClient, ratings RatingsProvider, cacheCfg CacheConfig) *Client` (or accept a functional-options `Option` to keep backward compat — pick one and document; the codebase uses plain configs, so the explicit param fits the style)
- [ ] update `cmd/lazysoap/main.go` to read `cfg.TVMeta.Cache` and pass to `tvmeta.New`
- [ ] add a top-level `TVMeta` struct in `internal/pkg/config/config.go` with a `Cache CacheConfig` field, wired into the existing `Config` tree
- [ ] update `config/config.yaml.dist` with a `tvmeta:` block mirroring the defaults
- [ ] update tests:
  - existing `New(...)` callers in tests pass a zero `CacheConfig{}` (which yields disabled-cache pass-through behavior — keeps existing tests deterministic without cache interference)
  - add a config-parsing test if `internal/pkg/config/config.go` already has one (check first; if yes, extend it)
- [ ] run `make test` and `make lint` — both must pass before next task

### Task 7: Cache observability (Prometheus counters)
- [ ] add three counter vectors in `internal/pkg/tvmeta/cache.go`, labeled by `method` (one of `details|episodes|search`): `tvmeta_cache_hits_total`, `tvmeta_cache_misses_total`, `tvmeta_cache_errors_total`
- [ ] register the metrics with `prometheus.DefaultRegisterer` in a `sync.Once` so multiple `New` calls in tests don't double-register; OR accept a `prometheus.Registerer` parameter (preferred — matches the existing `rest.NewMetrics()` pattern). Pick the latter.
- [ ] increment counters from `responseCache.GetOrFetch` (label = method name passed in at construction, e.g. `newResponseCache[…]("details", size, ttl, registerer)`)
- [ ] update tests:
  - hit/miss counters increment correctly under known sequences (use a `prometheus.NewRegistry` per test for isolation; assert via `testutil.ToFloat64`)
- [ ] run `make test` — must pass before next task

### Task 8: Update README, CLAUDE.md, OpenAPI
- [ ] README.md: extend the "Architecture" / "Configuration" section with a paragraph on the new response cache (what is cached, default TTLs, knobs). Be precise about *what's not cached* (popular pool, image proxy). Numbers come from the actual code, not from this plan — re-read `cache.go` defaults before writing.
- [ ] README.md: do NOT promise specific latency numbers until Task 9 measures them. Leave a `TODO: measure` placeholder if needed; remove after Task 9.
- [ ] CLAUDE.md: append a sub-bullet to the "Two pieces of non-obvious behavior" list (which becomes three) describing the response cache, with the same shared-read-only warning that the existing `featuredExtras.view()` bullet uses. Document the deep-copy-before-mutate requirement for the episodes/search code paths.
- [ ] `api/openapi.yaml`: this change does not alter the public API contract (response shapes unchanged, no new endpoints) — confirm no edits needed; if the spec mentions caching behavior in prose, sync it.
- [ ] no new tests for docs, but verify `make lint` / `make test` still pass — must pass before next task

### Task 9: Verify acceptance criteria
- [ ] verify all goals from Overview are implemented:
  - `/id/{id}` warm-hit issues zero TMDB calls for `details` + `episodes` (verified at unit-test layer in Tasks 3–4; spot-check via TMDB request log on the running container)
  - global TMDB call volume drops measurably for repeated requests (same)
- [ ] run `make lint` — all issues must be fixed
- [ ] run `make test-race ./...` — no races, all green
- [ ] run `make test-cov` — coverage on new files (`cache.go`) at or above project standard (project doesn't pin a target; aim ≥80% on new code)
- [ ] build and run the full stack:
  ```sh
  make docker-build
  docker compose up -d
  until curl -fsS http://127.0.0.1:8202/ping >/dev/null; do sleep 1; done
  ```
- [ ] measure cold vs warm latency on `/id/{id}` for a many-season show (e.g. Game of Thrones id=1399, 8 seasons):
  ```sh
  curl -fsS -o /dev/null -w "cold: %{time_total}s\n" 'http://127.0.0.1:8202/id/1399?language=en'
  curl -fsS -o /dev/null -w "warm: %{time_total}s\n" 'http://127.0.0.1:8202/id/1399?language=en'
  curl -fsS -o /dev/null -w "warm: %{time_total}s\n" 'http://127.0.0.1:8202/id/1399?language=en'
  ```
- [ ] capture the actual numbers and update README/CLAUDE.md if any prose claims a specific factor improvement (per CLAUDE.md "Performance numbers come from measurement")
- [ ] also exercise `/id/1399?language=ru` once (different lang key) to confirm localization keys do not collide
- [ ] `docker compose down`

### Task 10: [Final] Plan & doc cleanup
- [ ] confirm README.md, CLAUDE.md, and `config/config.yaml.dist` all reflect final defaults and numbers
- [ ] confirm `api/openapi.yaml` is unchanged (or updated if any wording referenced prior cache state)
- [ ] confirm `go.mod` / `go.sum` / `vendor/` are tidy (`make tidy` is a no-op)

*Note: ralphex automatically moves completed plans to `docs/plans/completed/`.*

## Technical Details

**Cache primitive shape**
```go
type responseCache[K comparable, V any] struct {
    name string                         // metric label
    lru  *expirable.LRU[K, V]           // nil when disabled
    sf   singleflight.Group
    m    *cacheMetrics                  // hit/miss/err counters
}

func (c *responseCache[K, V]) GetOrFetch(
    ctx context.Context,
    key K,
    fetch func(ctx context.Context) (V, error),
) (V, error)
```
- Disabled mode (`lru == nil`): every call hits `fetch`, no singleflight (effectively a pass-through), preserves test determinism for callers that pass `CacheConfig{}`.
- Singleflight key derivation: `fmt.Sprint(key)` for `comparable` keys is acceptable; alternatively define a `String()` on the typed key. Prefer the latter for keys with multiple fields (avoids `%v` ambiguity between `{1, 2, ""}` and `{1 2 }`).

**Cache keys**
| Method | Key |
| --- | --- |
| `TVShowDetails` | `{ id int; lang string }` |
| `TVShowEpisodesBySeason` | `{ id, season int; lang string }` |
| `SearchTVShows` (raw) | `{ query, lang string }` (`lang = languageTag(query)`) |

**TTL & size defaults** (configurable via env / yaml):
- Details: 1024 entries, 6h
- Episodes: 4096 entries, 6h
- Search: 256 entries, 30m

Memory footprint estimate (rough; revise after Task 9 measurement):
- Details: ~500B/entry × 1024 ≈ 0.5 MB
- Episodes: ~2 KB/entry (10–20 episode structs) × 4096 ≈ 8 MB
- Search: ~5 KB/entry (20 series structs) × 256 ≈ 1.3 MB
- Total cap: ~10 MB resident on top of current heap.

**Why these TTLs:**
- Details (6h): series metadata changes slowly; 6h trades off staleness against featured-extras refresh (24h) and is shorter than typical between-deploy intervals.
- Episodes (6h): per-season episode data is even more stable; tied to details TTL for symmetry.
- Search (30m): query results carry ratings overrides that the IMDb provider refreshes every 24h; 30m bounds the staleness of derived data while still capturing within-session repeat searches.

**Failure modes & mitigations:**
- TMDB error: not cached (next call re-tries immediately).
- TMDB timeout under load: singleflight collapses concurrent identical fetches into one; the configured 10s `TMDB_REQUEST_TIMEOUT` still bounds the worst case; subsequent waiters get the timeout error and re-try on the next request.
- Process restart: cache is lost (intentional, in-memory only). First requests after restart see today's behavior; tolerable.
- Stale data after TMDB-side correction: bounded by per-method TTL.

## Post-Completion

*Items requiring manual intervention or external systems — no checkboxes, informational only.*

**Manual verification:**
- After deploy, watch `tvmeta_cache_hits_total` / `tvmeta_cache_misses_total` ratios for the first day. Hit ratio < 30% on `episodes` after warm-up means TTLs are too short or sizes are too small for production traffic; tune via env vars without redeploying code.
- Confirm `docker stats` resident memory increase is in line with the ~10 MB estimate from Technical Details. If much higher, profile with `/debug/pprof/heap` and revisit cache sizes.
- Watch for any user-visible staleness reports (e.g., a series air date appears wrong for hours after TMDB update). If reports come in, lower `TVMETA_CACHE_DETAILS_TTL` first.

**External system updates:**
- None. The change is internal; no consuming projects, no schema, no deploy config.

**Future work (out of scope here):**
- Persistent cache (badger/bbolt on disk) to survive restarts — only worth it if cold-start TMDB load becomes a real concern.
- Per-method "stale-while-revalidate" (serve stale, refresh in background) — only worth it if TMDB latency dominates p99 in production.
- HTTP transport-level cache as a safety net under tvmeta — only worth it if other TMDB calls grow beyond the four currently used.
