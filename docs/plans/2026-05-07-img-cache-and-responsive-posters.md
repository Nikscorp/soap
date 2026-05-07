# Image cache + prewarm + responsive poster `srcset`

## Overview

Two related changes to make the home page paint faster without dropping poster quality:

1. **Server-side bytes cache for `/img`** with a prewarmer that keeps every featured-pool poster (curated extras *and* popular) hot at every size the SPA actually requests. The `/img` handler is currently a pure pass-through — every cold poster pays a TMDB round-trip, which dominates first-paint latency on `/featured`. The new cache is built on top of the existing generic LRU+metrics harness already living in `internal/pkg/tvmeta/cache.go`, promoted to a shared `internal/pkg/lrucache` package so both the TMDB response cache and the new image bytes cache share one tested implementation.
2. **Responsive `srcset` + `sizes`** on the three poster components in the SPA so mobile devices stop downloading desktop-sized JPEGs. Right now `SearchResultCard` ships `w92` (too small), `FeaturedCard` ships `w342`/`w500` 1x/2x with no `sizes` (so a phone gets the same bytes as a desktop), and `SelectedSeriesCard` is in the same shape.

Net result: featured page bytes drop on phones (smaller renditions selected) and TMDB latency disappears from the request path for any poster in the featured pool.

## Context (from discovery)

**Backend files involved:**
- `internal/app/lazysoap/img_proxy.go:12-45` — `Server.imgProxyHandler()`, current pass-through.
- `internal/app/lazysoap/featured.go` — `featuredExtrasCache` (lines 43-45), `runFeaturedExtrasRefresh()` (lines 145-167), `collectFeaturedPool()` (lines 105-119), `/featured` handler (lines 74-98).
- `internal/app/lazysoap/server.go` — `Config` (lines 30-42), `ImgClientConfig` (lines 38-42), HTTP client construction (lines 65-77), goroutine kickoff at line 98, `tvMetaClient` interface (lines 53-58).
- `internal/pkg/tvmeta/cache.go` — generic `responseCache[K, V]` + `cacheMetrics` already implements expirable LRU + singleflight + Prometheus metrics + pass-through-on-zero-config + idempotent metric registration. Promoted to `internal/pkg/lrucache` in Task 1 and reused by both tvmeta and the new img cache. `golang-lru/v2 v2.0.7` is already on `go.mod` (line 12) — no new dep needed.
- `internal/pkg/tvmeta/poster.go:10-26` — `DefaultPosterSize`, allow-list, `GetURLByPosterPathWithSize()`.
- `internal/pkg/config/config.go` + `config/config.yaml.dist` — config wiring per CLAUDE.md.

**Frontend files involved:**
- `frontend/src/components/FeaturedSeries.tsx:79-124` — `FeaturedCard`, current 1x/2x `srcSet` (w342, w500), `loading="lazy"`. Container `w-40` (~160px) on mobile, `sm:w-auto` (~190–210px) on desktop, `aspect-[2/3]`.
- `frontend/src/components/SearchResultCard.tsx:12-62` — no `srcSet`, defaults to w92, `h-[120px] w-[80px]` mobile, `sm:h-[135px] sm:w-[90px]` desktop.
- `frontend/src/components/SelectedSeriesCard.tsx:14-62` — 1x/2x w185/w342, `loading="lazy"`. Container `w-20` mobile, `sm:w-24` desktop.
- `frontend/src/lib/api.ts:94-103` — `normalizePosterUrl()` builds `/img/{path}?size=…` URLs. The natural place for any new helper that builds an `srcset` string.

**Tests + mocks:**
- `internal/pkg/tvmeta/cache_test.go` — coverage for `responseCache[K, V]` + `cacheMetrics`. Most of these tests follow `responseCache` to its new home in Task 1.
- `internal/app/lazysoap/img_proxy_test.go` — 7 cases over the proxy handler.
- `internal/app/lazysoap/featured_test.go` — 10 functions; `TestRunFeaturedExtrasRefreshTicks` (lines 321-346) already exercises the refresh goroutine.
- `internal/app/lazysoap/mocks/TvMetaClientMock` — generated; `tvMetaClient` is the seam.
- Frontend: Vitest+RTL component tests live next to each component; one Playwright smoke test in `frontend/e2e/`.

**Decisions already made (from planning conversation + revdiff review):**
- Cache backend: in-memory expirable LRU (no disk).
- LRU + metrics infra: extracted to `internal/pkg/lrucache` and reused by both `internal/pkg/tvmeta` and the new `imgCache` (Task 1). `tvmeta_cache_*` Prometheus metric names stay byte-identical so existing dashboards don't break; img cache emits a parallel `lazysoap_img_cache_*` family via the same `lrucache.NewMetrics(reg, prefix, subject)` helper.
- Popular handling: cache popular alongside extras in a single unified pool refreshed on the existing interval. `/featured` reads from the cached pool; no live `PopularTVShows()` call per request.
- Prewarm sizes: `w185, w342, w500, w780` (fixed const, not env-configurable — user explicitly chose the fixed-list option over the configurable one).
- Testing: regular (code first, then tests).

## Development Approach

- **Testing approach**: regular (code first, tests follow within the same task)
- Complete each task fully before moving to the next.
- **CRITICAL: every task MUST include new/updated tests** for the code changes in that task. Tests are not optional.
  - unit tests for new functions, new test cases for new branches, updated tests for modified behavior
  - cover both success and error paths
- **CRITICAL: all tests must pass before starting the next task** — no exceptions.
- **CRITICAL: update this plan file when scope changes during implementation.**
- Run `make test` (and the relevant frontend test where applicable) after each change.
- Maintain backward compatibility on JSON response shapes — only headers and timing should change.

## Testing Strategy

- **Unit tests** (Go): required per task; mirror existing minimock-driven patterns in `internal/app/lazysoap/*_test.go`. Use `-race` when testing the prewarmer or atomic-swap paths.
- **Component tests** (Vitest+RTL): required for each frontend component touched. Assert that the rendered `<img>` carries `srcset`, `sizes`, and the right `loading`/`fetchpriority` attributes.
- **E2E test** (Playwright): the existing smoke test in `frontend/e2e/` should still pass; add a check that the home-page hero poster has `fetchpriority="high"` if a smoke assertion is cheap to add.
- **Integration via Docker** (in Task N-1 only): `make docker-build` + `docker compose up -d`, hit `/featured` cold, confirm cache-fill on second hit, and measure bytes-on-the-wire at iPhone widths.

## Progress Tracking

- Mark completed items with `[x]` immediately when done.
- Add newly discovered tasks with ➕ prefix.
- Document issues/blockers with ⚠️ prefix.
- Update plan if implementation deviates from original scope.

## What Goes Where

- **Implementation Steps** (`[ ]` checkboxes): code, tests, docs the agent can do inside the repo.
- **Post-Completion** (no checkboxes): manual browser verification, real-device checks, deploy considerations.

## Implementation Steps

### Task 1: Extract `internal/pkg/lrucache` from `internal/pkg/tvmeta/cache.go`

The existing `responseCache[K, V]` + `cacheMetrics` in `internal/pkg/tvmeta/cache.go` already does everything the new `/img` cache needs. Promote them to a shared package so the refactor is one-time and `imgCache` (Task 2) reuses the entire harness — including singleflight (collapses concurrent misses for the same poster to one TMDB fetch), pass-through-on-zero-config, and the idempotent `AlreadyRegisteredError` branch in metric registration.

- [ ] confirm `go.mod` already lists `github.com/hashicorp/golang-lru/v2 v2.0.7` (it does, line 12) — no `go get` needed.
- [ ] create `internal/pkg/lrucache/cache.go`:
  - export `Cache[K comparable, V any]` (renamed from `responseCache[K, V]`) — same fields, same `GetOrFetch`/`Len`/recordHit/Miss/Error methods. Document that cached values are SHARED and read-only (carry over the existing comment).
  - export `Metrics` (renamed from `cacheMetrics`) and `NewMetrics(registerer prometheus.Registerer, namePrefix, helpSubject string) *Metrics`. The `namePrefix` controls the metric-family name (`{namePrefix}_hits_total`, etc.); `helpSubject` is the noun in help text (e.g., `"TMDB response cache"` or `"image bytes cache"`). nil registerer → nil Metrics, all record* calls remain nil-safe.
  - export `New[K, V](name string, size int, ttl time.Duration, m *Metrics) *Cache[K, V]` — same pass-through-on-zero-config semantics as today.
  - export `ErrTypeAssert` (renamed from `errCacheTypeAssert`).
  - keep the `method` Prometheus label name (constant `"method"`) so existing label cardinality stays identical.
- [ ] move tests from `internal/pkg/tvmeta/cache_test.go` to `internal/pkg/lrucache/cache_test.go` for everything that exercises generic cache mechanics (hit/miss, pass-through, singleflight collapse, ctx cancellation, idempotent register). Keep tvmeta-specific tests (key shapes, per-method config plumbing) in tvmeta.
- [ ] refactor `internal/pkg/tvmeta/cache.go` to thin-wrap `lrucache`:
  - `tvmeta.CacheConfig` stays unchanged (the `TVMETA_CACHE_*` env-tag contract is tvmeta's, not lrucache's).
  - `newCacheMetrics(reg)` becomes `lrucache.NewMetrics(reg, "tvmeta_cache", "TMDB response cache")`.
  - per-method cache constructors (details/all_seasons/search) become one-line calls to `lrucache.New[K, V](name, size, ttl, m)`.
- [ ] verify the resulting Prometheus metric names are byte-identical to before — namely `tvmeta_cache_hits_total{method="details|all_seasons|search"}`, `_misses_total`, `_errors_total`. Add (or keep) a test that gathers from a registry and asserts the family names + label values, since dashboards depend on them and a silent rename would be invisible until a panel goes blank.
- [ ] run `go test -race ./internal/pkg/lrucache/... ./internal/pkg/tvmeta/...` — must pass before Task 2.

### Task 2: `imgCache` type + entry shape (thin layer on `lrucache.Cache`)

- [ ] create `internal/app/lazysoap/img_cache.go`:
  - `type imgCacheEntry struct { body []byte; contentType string }` (immutable after store; comment that `body` is shared with all readers and MUST NOT be mutated — same contract as `lrucache.Cache` values, called out at the use site).
  - `type imgCache = *lrucache.Cache[string, imgCacheEntry]` (type alias — no wrapper struct unless ergonomic helpers are needed).
  - helper `imgCacheKey(path, size string) string` returning `path + "|" + size`. Document that `size` MUST be the post-allow-list-normalization value so `?size=garbage` and the default share a slot.
- [ ] tests in `internal/app/lazysoap/img_cache_test.go`:
  - keep these focused on the *wiring*, since cache mechanics are already covered upstream in `internal/pkg/lrucache/cache_test.go`.
  - `imgCacheKey` round-trips and is collision-free across plausible inputs (paths starting with `/`, sizes from the allow-list).
  - constructing the cache via `lrucache.New[string, imgCacheEntry]` with a real `lrucache.Metrics` produces the expected Prometheus family name (`lazysoap_img_cache_hits_total`).
- [ ] run `go test -race ./internal/app/lazysoap/...` — must pass before Task 3.

### Task 3: Wire `imgCache` into the `/img` proxy + add `Cache-Control`

- [ ] add `ImgCacheConfig` struct to `server.go` next to `ImgClientConfig`:
  - `Size int` env `LAZYSOAP_IMG_CACHE_SIZE` env-default `512` yaml `size`.
  - `TTL time.Duration` env `LAZYSOAP_IMG_CACHE_TTL` env-default `168h` yaml `ttl` (TMDB image paths are content-addressed, so a long TTL is safe; cap exists so a moved/dead path eventually rotates out).
  - `BrowserMaxAge time.Duration` env `LAZYSOAP_IMG_BROWSER_MAX_AGE` env-default `86400s` yaml `browser_max_age` (controls the `Cache-Control: public, max-age=…, immutable` we send to the SPA).
- [ ] add `ImgCache ImgCacheConfig` field on the top-level `Config` struct in `cmd/lazysoap/main.go` (or wherever the top-level struct lives — confirm during implementation).
- [ ] in `cmd/lazysoap/main.go`, build a single `*lrucache.Metrics` for the img family via `lrucache.NewMetrics(reg, "lazysoap_img_cache", "image bytes cache")`, then construct the `imgCache` via `lrucache.New[string, imgCacheEntry]("img", cfg.ImgCache.Size, cfg.ImgCache.TTL, imgMetrics)`. Pass it into `lazysoap.New()`; store on `Server`.
- [ ] mirror the new fields in `config/config.yaml.dist` with comments.
- [ ] modify `imgProxyHandler` (`internal/app/lazysoap/img_proxy.go`):
  - normalize size to the allow-list *before* the cache lookup (use the existing `tvmeta.GetURLByPosterPathWithSize` normalization logic — extract into a helper if it isn't already exported).
  - use `imgCache.GetOrFetch(ctx, imgCacheKey(path, size), func(ctx) (imgCacheEntry, error) { … })` so concurrent misses for the same poster collapse to one TMDB fetch (singleflight comes free with `lrucache`).
  - inside the fetch closure: hit TMDB via the existing `s.imgClient`, `io.ReadAll` the body (size-bounded — refuse > 2 MB; w780 caps near 250 KB so 2 MB is generous; oversized → return an error so the closure aborts without populating the cache).
  - only return an `imgCacheEntry` (i.e., populate the cache) on `200 OK` with `Content-Type: image/*` and a non-empty body. Anything else returns an error from the fetch closure — `lrucache` does not cache errors (`responseCache` already enforces this; carry the contract over verbatim).
  - on success path: write `Content-Type` from the entry, `Cache-Control: public, max-age={BrowserMaxAge.Seconds()}, immutable`, then the body. Same headers on hit and miss-success.
  - on error path: map the cache fetch error back to the appropriate HTTP status (404 if upstream returned 404, 502 if upstream errored, 502 for oversized/non-image).
- [ ] update `internal/app/lazysoap/img_proxy_test.go`:
  - existing 7 cases stay green (some will need a non-zero `imgCache` injected via the `Server` test fixture; pass an enabled `lrucache.New` instance with no metrics).
  - new test: second request for the same `(path, size)` does not hit the upstream (assert mock-roundtripper call count == 1 across two sequential requests).
  - new test: concurrent requests for the same key collapse to one upstream call (singleflight verification — N goroutines, assert call count == 1).
  - new test: oversized-body upstream produces a 502, no cache write (re-request must fetch again).
  - new test: non-image content-type from upstream is not cached.
  - new test: `Cache-Control: public, max-age=…, immutable` is present on both hits and misses.
- [ ] run `go test -race ./internal/app/lazysoap/...` — must pass before Task 4.

### Task 4: Cache popular alongside extras in a single unified pool

- [ ] rename `featuredExtrasCache` → `featuredPoolCache` in `internal/app/lazysoap/featured.go` (or add a parallel type if rename churn is too noisy — decide during implementation, prefer rename).
- [ ] change the `featuredItem` slice the cache holds to be the *unified* pool (curated extras ∪ deduped popular). Keep dedup-by-ID semantics that already live in `collectFeaturedPool`.
- [ ] rewrite `runFeaturedExtrasRefresh` to:
  - call `PopularTVShows()` and `refreshFeaturedExtras()`'s TMDB lookups in the same refresh tick.
  - build the unified pool (popular first, extras appended, dedup-by-ID, vote-count filter applied to popular only as today).
  - atomic-swap the unified slice.
  - on partial failure (popular fetch fails, extras succeed, or vice-versa), keep whichever side succeeded; on total failure, keep the prior pool (do not blank it).
- [ ] simplify `/featured` handler (lines 74-98) to read directly from `featuredPool.view()` instead of calling `collectFeaturedPool` per request. The 503-when-pool-too-small branch stays. `Cache-Control: no-store` stays.
- [ ] delete `collectFeaturedPool` if no other call site remains; keep otherwise.
- [ ] update `featured_test.go`:
  - existing 10 test functions pass (some assertions against the live `PopularTVShows` mock will move into the refresh-cycle setup).
  - new test: refresh populates a unified pool of popular ∪ extras with dedup.
  - new test: popular-fetch failure preserves prior pool (assert `view()` returns the previously-warmed slice).
  - new test: extras-fetch failure preserves prior pool symmetrically.
  - new test: `/featured` no longer calls `PopularTVShows` per request once the pool is warm (assert mock call count after N requests stays at the refresh count).
- [ ] run `go test -race ./internal/app/lazysoap/...` — must pass before Task 5.

### Task 5: Prewarm `/img` cache after each pool refresh

- [ ] add `prewarmSizes = []string{"w185", "w342", "w500", "w780"}` as an unexported `var` near the top of `featured.go` (fixed list per planning decision; if you find a need to make this configurable, raise it as a ➕ task instead of expanding scope silently).
- [ ] add a method `(s *Server) prewarmFeaturedImages(ctx context.Context, pool []featuredItem)` that:
  - for each `(item, size)` pair where `item.Poster != ""`, calls `imgCache.GetOrFetch(ctx, imgCacheKey(path, size), s.fetchPosterBytes)` to warm the cache through the same code path the live handler uses (extract a private `fetchPosterBytes(ctx, path, size)` helper from `imgProxyHandler` so prewarmer and handler share one fetch closure).
  - bounds concurrency with `errgroup.SetLimit(8)` to avoid slamming TMDB.
  - logs one summary line at the end (`prewarm: warmed N/M entries in T`).
  - never returns an error to the caller — individual failures are logged and swallowed; one bad poster must not block the rest.
- [ ] hook the prewarmer into the refresh loop: after every successful pool swap (initial + each tick), kick off `go s.prewarmFeaturedImages(ctx, pool)` so it doesn't block the next tick.
- [ ] write tests in `featured_test.go`:
  - prewarm populates the `imgCache` with `len(pool) * len(prewarmSizes)` entries on the happy path (use the existing `TvMetaClientMock` to return curated items + a stub HTTP roundtripper for the proxy fetch). Assert `cache.Len() == expected`.
  - prewarm with one TMDB-failing poster still warms the rest.
  - prewarm respects `ctx` cancellation (cancel mid-warm, assert it returns promptly).
- [ ] run `go test -race ./internal/app/lazysoap/...` — must pass before Task 6.

### Task 6: Responsive `srcset` + `sizes` for `FeaturedCard`

- [ ] add a helper in `frontend/src/lib/api.ts` (next to `normalizePosterUrl`):
  - `posterSrcSet(path: string | null, sizes: readonly Size[]): string` — returns a space/comma-separated string like `"/img/foo.jpg?size=w185 185w, /img/foo.jpg?size=w342 342w, …"` using *width* descriptors (not pixel-density) so the `sizes` attribute drives selection.
  - `Size` = `'w185' | 'w342' | 'w500' | 'w780'`.
  - empty/null path → empty string (caller falls back to its existing placeholder).
- [ ] update `FeaturedCard` (`frontend/src/components/FeaturedSeries.tsx:79-124`):
  - replace the existing 1x/2x `srcSet` with a width-descriptor `srcSet` covering w342, w500, w780 (skip w185 — too small for this card even on mobile).
  - add `sizes="(min-width: 1024px) 210px, (min-width: 640px) 190px, 160px"` matching the Tailwind container widths discovered in the map.
  - keep `loading="lazy"` on cards 3+ in the carousel; set `loading="eager"` and `fetchpriority="high"` on the first 1–2 cards (the ones above the fold on a phone). Determine the index threshold from the existing carousel code.
- [ ] update the matching Vitest test (or create one if absent under `frontend/src/components/`): assert `srcset` contains all four URL/width pairs, `sizes` matches the breakpoint string, and the first card has `fetchpriority="high"`.
- [ ] run `npm run lint && npm run typecheck && npm run test:ci` (from `frontend/`) — must pass before Task 7.

### Task 7: Responsive `srcset` + `sizes` for `SearchResultCard`

- [ ] update `SearchResultCard` (`frontend/src/components/SearchResultCard.tsx:12-62`):
  - swap the bare `normalizePosterUrl(result.poster)` for an explicit `src="…?size=w185"` plus `srcSet` with w185 + w342 (no need for w500/w780 — the card maxes at ~90px wide).
  - add `sizes="(min-width: 640px) 90px, 80px"` matching Tailwind classes.
  - keep `loading="lazy"` (search results are below the fold by definition).
- [ ] update / add the Vitest test: `srcset` has both URLs with the correct width descriptors, `sizes` is set, `loading="lazy"` is preserved.
- [ ] run frontend lint+typecheck+test — must pass before Task 8.

### Task 8: Responsive `srcset` + `sizes` for `SelectedSeriesCard`

- [ ] update `SelectedSeriesCard` (`frontend/src/components/SelectedSeriesCard.tsx:14-62`):
  - replace the 1x/2x `srcSet` with width descriptors over w185, w342 (max ~96px wide; w500 is overkill).
  - add `sizes="(min-width: 640px) 96px, 80px"`.
  - drop `loading="lazy"` and add `fetchpriority="high"` — this is the hero poster on the detail page; deprioritizing it costs perceived performance.
- [ ] update / add the Vitest test: `srcset`, `sizes`, and `fetchpriority="high"` are present; `loading` is *not* `"lazy"`.
- [ ] run frontend lint+typecheck+test — must pass before Task 9.

### Task 9: Verify acceptance criteria (lint, integration, measurement)

- [ ] `make lint-all` — covers `golangci-lint`, frontend `eslint`, frontend `typecheck`. CLAUDE.md flags this as required for every change. Fix every issue before continuing.
- [ ] `make test-race` — full backend suite under `-race`.
- [ ] `npm run test:ci` and `npm run test:e2e` (from `frontend/`) — full frontend + Playwright smoke.
- [ ] `make docker-build` (foreground with `timeout: 180000` per CLAUDE.md) → `docker compose up -d` → `until curl -fsS http://127.0.0.1:8202/ping >/dev/null; do sleep 1; done`.
- [ ] sanity-curl the integrated stack:
  - `curl -fsS 'http://127.0.0.1:8202/featured?language=en' | jq '. | length'` — confirm pool returns ≥ `FeaturedCount`.
  - `curl -fsS 'http://127.0.0.1:8202/featured?language=ru' | jq .` — confirm localization path works.
  - `curl -sI 'http://127.0.0.1:8202/img/<path>?size=w500'` — confirm `Cache-Control: public, max-age=…, immutable`.
  - hit the same `/img` URL twice with `time curl …` — second request must be measurably faster (cache hit).
  - check container logs for the `prewarm: warmed N/M …` summary line and confirm `N == M` for a healthy run.
  - `curl -fsS 'http://127.0.0.1:8202/metrics' | grep -E '(tvmeta_cache|lazysoap_img_cache)_(hits|misses|errors)_total'` — confirm both metric families exist with the expected `method` labels.
- [ ] **measure poster bytes** at iPhone width vs desktop width using Chrome DevTools' Network panel with iOS Safari UA (per CLAUDE.md "performance numbers from measurement, not estimation"):
  - record total bytes for `/featured` poster requests at `iPhone 14` UA + 390px width vs `Desktop` at 1440px width, before and after.
  - capture the numbers in the completion report (e.g. "phone: 412 KB → 168 KB; desktop: 412 KB → 412 KB").
- [ ] `docker compose down`.

### Task 10: [Final] Documentation

- [ ] update `README.md` `/img` section to describe the new server-side cache + `Cache-Control` headers it emits. Re-derive the prose by re-reading the implementation per CLAUDE.md "documentation discipline".
- [ ] update `README.md` `/featured` section to note that the popular pool is now cached (refreshed on `LAZYSOAP_FEATURED_EXTRAS_REFRESH_INTERVAL`) rather than fetched per request, and that featured posters are prewarmed.
- [ ] update `api/openapi.yaml` if any header/parameter description for `/img` or `/featured` changes (no schema changes expected — verify response shapes are byte-identical).
- [ ] confirm `config/config.yaml.dist` mirrors the three new `LAZYSOAP_IMG_CACHE_*` / `LAZYSOAP_IMG_BROWSER_MAX_AGE` env vars added in Task 3 with comments.
- [ ] mention the new `internal/pkg/lrucache` package in CLAUDE.md if a one-liner under "Architecture" clarifies the layering — only if it would have helped this session's discovery; otherwise leave alone.

## Technical Details

**Shared cache harness (`internal/pkg/lrucache`).** Generic `Cache[K comparable, V any]` over `expirable.LRU` + singleflight + Prometheus metrics + pass-through-on-zero-config. Constructed via `lrucache.New(name, size, ttl, *Metrics)`; metrics constructed once per family via `lrucache.NewMetrics(reg, namePrefix, helpSubject)`. tvmeta keeps its `tvmeta_cache_*` family; img cache uses `lazysoap_img_cache_*`. The `method` label distinguishes caches inside one family. Idempotent `AlreadyRegisteredError` handling preserved verbatim.

**Cache key shape (img).** `path|size` where `size` is the post-normalization allow-list value (so `?size=garbage` and `?size=w92` share an entry). String concat is fine; collisions impossible because TMDB poster paths start with `/` and never contain `|`.

**Cache entry shape (img).** `{ body []byte, contentType string }` — bodies are typically 10–250 KB; storing copies is fine at our scale (~120 prewarm entries + headroom = 512 LRU cap default). Body slice is shared with all readers and read-only — same contract as `lrucache.Cache` values generally.

**Memory budget (estimate; verify in Task 9).** ~30 featured posters × 4 sizes × ~75 KB avg ≈ 9 MB resident for the prewarmed set. Cap at 512 entries to leave headroom for non-featured posters served on detail/search pages.

**Refresh + prewarm timing.** Initial refresh fires from a goroutine in `Server.Run` (existing pattern, intentionally async so a slow TMDB at boot doesn't block `ListenAndServe`). Prewarm fires after each successful pool swap, bounded to 8 concurrent fetches via `errgroup.SetLimit(8)`. `/featured` returns 503 until the first pool swap completes — same as today.

**Cache-Control on `/img`.** Hits and misses both set `Cache-Control: public, max-age={BrowserMaxAge.Seconds()}, immutable`. Default 24h browser cache is conservative — TMDB image paths are content-addressed in practice, so longer is safe, but 24h gives us a knob without committing to "forever".

**`srcset` / `sizes` strategy.** Use *width descriptors* (`w`), not pixel-density (`1x/2x`). Pixel-density forces the browser to assume a fixed CSS size; width descriptors let `sizes` express the breakpoint reality and let the browser pick the smallest rendition that satisfies actual layout × DPR. This is the change that lets a 390px phone download w342 instead of w500.

**Sizes per component:**
- `FeaturedCard`: w342, w500, w780 (`sizes="(min-width: 1024px) 210px, (min-width: 640px) 190px, 160px"`)
- `SearchResultCard`: w185, w342 (`sizes="(min-width: 640px) 90px, 80px"`)
- `SelectedSeriesCard`: w185, w342 (`sizes="(min-width: 640px) 96px, 80px"`)

**Loading priority.**
- `FeaturedCard` first 1–2: `loading="eager"` + `fetchpriority="high"`. Rest of carousel: `loading="lazy"` (unchanged).
- `SearchResultCard`: `loading="lazy"` (unchanged).
- `SelectedSeriesCard`: `fetchpriority="high"`, no `loading="lazy"` (hero poster).

**Backwards compatibility.** No JSON shapes change. `tvmeta_cache_*` Prometheus metric names stay byte-identical (verified explicitly in Task 1) so existing dashboards keep working. `/featured` already had `Cache-Control: no-store`; that stays. `/img` gains `Cache-Control` headers it didn't set before — that is a behavior change (browsers will now cache), but a benign one since TMDB paths don't mutate.

## Post-Completion

*Items requiring manual intervention or external systems — informational only, no checkboxes.*

**Manual browser verification** (per CLAUDE.md "frontend verification gotchas"):
- Open the SPA on a real iOS Safari device (not Chrome desktop with iPhone UA — macOS Chrome clamps width to ~500px and won't reproduce a 390px viewport faithfully).
- Confirm featured carousel posters appear sharp (no upscaling artifacts) and the home page paints visibly faster than master.
- Pull DevTools Network panel: confirm phone gets `w342` (or `w500` on retina) for featured cards, `w185` for search and detail thumbnails.
- After deploy, hard-reload via `navigator.serviceWorker.getRegistrations().then(rs => Promise.all(rs.map(r => r.unregister())))` + `caches.keys().then(ks => Promise.all(ks.map(k => caches.delete(k))))` so the service worker doesn't serve a stale shell — that gotcha cost time before.

**Operational verification:**
- After deploy, watch `/metrics` for `lazysoap_img_cache_hits_total` and `_misses_total`. Hit rate on `/featured`-flow posters should approach 100% within one refresh interval. If it doesn't, the prewarmer is failing silently — check container logs for the `prewarm: warmed N/M …` summary.
- Watch container memory: expect ~10–15 MB rise vs master from the cached image bytes. If RSS climbs past +50 MB, bump `LAZYSOAP_IMG_CACHE_SIZE` down or check for unbounded `body` retention.
- Verify `tvmeta_cache_*` Prometheus families still scrape with the same shape after the lrucache extraction — any silent rename would only show up as a blank dashboard panel.

**External system updates:** none — this is a single-image deploy with no consuming projects.
