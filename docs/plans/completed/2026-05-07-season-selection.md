# Season selection for "best episodes"

## Overview

Let users pick a subset of seasons; only episodes from selected seasons are
considered when computing the "best" episodes (`defaultBest` and the limited
list). All seasons selected is the default and produces today's response
byte-for-byte (modulo the new `availableSeasons` field).

**Wire**

- New optional query param on `GET /id/{id}`: `seasons=1,3,5` (comma-separated,
  ascending unique). Param absent or empty → all seasons (current behavior).
- New response field `availableSeasons: number[]` listing every season number
  that exists for the series, regardless of filter. The frontend renders the
  selector chips from this — it's the only way the UI knows seasons that were
  filtered *out*.
- `defaultBest` and `totalEpisodes` are recomputed over the filtered subset:
  slider max = episodes in selection, `defaultBest` = top-quantile-floor over
  the selection. Empty selection (no valid seasons after intersection with
  available) → 400.

**Server**

- `idHandler` parses `seasons`, intersects with the seasons returned by the
  cached `TVShowAllSeasonsWithDetails`, then filters the flatten step. The
  `tvmeta` layer is unchanged: filtering happens *on top of* the cached
  read-only `*AllSeasonsWithDetails`, so the `all_seasons` cache key stays
  series-wide and shared. No mutation of the cached slice — the handler
  already allocates a fresh `[]episode` in `flattenSortedByRating`.

**Frontend**

- `useUrlState` gains a `seasons: number[] | null` field; `null` means "all
  selected" and is omitted from the URL (mirrors how `best` is omitted when
  it equals `defaultBest`).
- New `SeasonSelector` component: inline chip row above the existing slider.
  Tapping a chip toggles that season; an "All" chip selects/clears every
  season at once.
- `EpisodesList` adds the selection to its React Query key so a change
  triggers a refetch. Crucially, the monotone-in-N invariant the slider
  exploits ("a larger response always contains every episode of a smaller
  one") only holds within a fixed season set — selection changes must reset
  `fetchLimit` and the slider to the server-default position.

## Context (from discovery)

**Files involved**

- Backend
  - `internal/app/lazysoap/by_id.go` — `idHandler`, `flattenSortedByRating`,
    `computeDefaultBest`, response shape (`episodesResp`).
  - `internal/app/lazysoap/by_id_test.go` — handler/unit tests.
  - `internal/app/lazysoap/server.go` — router (no change expected; filter
    logic stays in `by_id.go`).
- API contract
  - `api/openapi.yaml` — `/id/{id}` parameters + `EpisodesResponse` schema.
- Docs
  - `README.md` — `/id/{id}` example block, `defaultBest` prose.
- Frontend
  - `frontend/src/hooks/useUrlState.ts` (+ `useUrlState.test.ts`) — URL state.
  - `frontend/src/lib/api.ts` (+ `api.test.ts`) — `getEpisodesById` signature.
  - `frontend/src/lib/types.ts` — `EpisodesResponse`.
  - `frontend/src/components/EpisodesList.tsx` (+ `EpisodesList.test.tsx`) —
    list/slider host; React Query key, refetch invariants.
  - `frontend/src/components/SeasonSelector.tsx` (new) +
    `SeasonSelector.test.tsx` (new).
  - `frontend/src/App.tsx` — wires `seasons` from `useUrlState` into
    `EpisodesList`.
- E2E
  - `frontend/e2e/` — extend the existing smoke test to exercise season
    toggling on a deterministic series.

**Cache touchpoint (do not touch)**

- `internal/pkg/tvmeta/cache.go` — the `all_seasons` cache holds the fully
  assembled `*AllSeasonsWithDetails` with IMDb overrides already applied,
  keyed by `(id, language)`. The handler reads it as a shared, read-only
  pointer. The filter pass must allocate fresh slices; no mutation,
  appending, or in-place sort on the cached value. Filtering is intentionally
  post-cache so the cache stays series-wide and there's no key explosion.

**Patterns found**

- `useUrlState` already owns canonical URL params (`q`, `id`, `lang`, `best`)
  with the "omit when default" pattern — extend it with `seasons` the same
  way.
- `EpisodesList`'s `fetchLimit` only ever grows. Season-set changes break
  that invariant; reset both `fetchLimit` and `sliderValue` on
  selection change.
- The chi router uses `s.tvMeta.TVShowAllSeasonsWithDetails(ctx, id, lang)`
  as the single load — keep that, filter the result.

## Development Approach

- **Testing approach**: Regular (code first, then tests in the same task).
  Matches existing style: table-driven Go tests, Vitest+RTL on the FE.
- Complete each task fully before moving to the next.
- Make small, focused changes.
- **CRITICAL: every task MUST include new/updated tests** for code changes
  in that task.
  - tests are not optional — they are a required part of the checklist.
  - write unit tests for new functions and modified functions.
  - cover both success and error scenarios.
- **CRITICAL: all tests must pass before starting the next task**.
- **CRITICAL: update this plan file when scope changes during implementation**.
- Run `make lint` and `make test` after each backend task; `npm run lint`
  / `npm run typecheck` / `npm run test:ci` after each FE task.
- Maintain backward compatibility — `seasons` absent must produce today's
  response shape (plus the new `availableSeasons` field).

## Testing Strategy

- **Unit tests**
  - Go: `parseSeasons` (valid CSV, empty, whitespace, invalid ints, dupes,
    out-of-range), `filterSeasons` (subset, all, none), `idHandler`
    (table-driven: no-filter golden, filtered subset, single-season,
    invalid filter, filter with no valid seasons → 400, defaultBest
    recomputes within subset, totalEpisodes reflects subset,
    `availableSeasons` always full).
  - TS: `useUrlState` round-trip (`seasons` parse/serialize/omit-when-null,
    sorting + dedup), `getEpisodesById` URL composition, `SeasonSelector`
    toggle/all-chip behaviour, `EpisodesList` refetch on selection change
    and slider reset.
- **E2E (Playwright smoke)**
  - Open a deterministic series (use the same fixture/series id the existing
    smoke test uses), toggle off one season, assert episodes from that
    season disappear and the slider's "X of Y" counter reflects a smaller
    `Y`, then re-select all and assert original counter is restored.
- **Manual via Docker** (Post-Completion section, not a checkbox): the full
  `make docker-build` + curl walk against live TMDB is enumerated in
  CLAUDE.md's "Before claiming a task done" — must be run for this change.

## Progress Tracking

- Mark completed items with `[x]` immediately when done.
- Add newly discovered tasks with ➕ prefix.
- Document issues/blockers with ⚠️ prefix.
- Update plan if implementation deviates from original scope.
- Keep plan in sync with actual work done.

## What Goes Where

- **Implementation Steps** (`[ ]` checkboxes): code, tests, docs that the
  agent can complete in-repo.
- **Post-Completion** (no checkboxes): manual `make docker-build` walk,
  browser verification on iOS Safari widths (per CLAUDE.md's frontend
  verification gotchas).
- **Checkbox placement**: only inside `### Task N:` sections.

## Implementation Steps

### Task 1: Backend — parse and validate `seasons` query param

- [x] add `parseSeasons(raw string) ([]int, bool)` helper in
      `internal/app/lazysoap/by_id.go`: returns the deduped, ascending-sorted
      list of positive ints from a comma-separated input; second return is
      `false` when the param is present-but-malformed (any non-int token, or
      a non-positive number) so the handler can 400. Empty / absent → `(nil,
      true)` meaning "no filter".
- [x] write table-driven tests in `by_id_test.go` covering: absent, empty,
      `"1"`, `"1,2,3"`, `"3,1,2"` (sorted), `"1,1,2"` (deduped), `"1, 2 ,3"`
      (whitespace tolerated), `"1,abc"` (invalid → false), `"0"` and `"-1"`
      (non-positive → false).
- [x] run `go test ./internal/app/lazysoap/... -race` — must pass.

### Task 2: Backend — filter step + `availableSeasons` in response

- [x] add `availableSeasons []int` field to `episodesResp` in `by_id.go`.
- [x] add a `filterSeasonsByNumber(seasons *tvmeta.AllSeasonsWithDetails,
      keep map[int]struct{}) []tvmeta.SeasonEpisodes`-style helper (or inline
      filter inside `flattenSortedByRating`) that copies only matching
      seasons. Allocate fresh slices — never mutate the cached input. The
      "all seasons" path stays the existing fast loop (no-op filter).
- [x] in `idHandler`: parse `seasons` (Task 1); on parse failure return 400
      with a logged error; compute `availableSeasons` from
      `seasons.Seasons[*].SeasonNumber` ascending sorted; if filter is
      non-empty, intersect with `availableSeasons` and 400 if intersection
      is empty; pass the keep-set to the filter step before
      `flattenSortedByRating`. `defaultBest` and `totalEpisodes` flow
      naturally from the filtered slice.
- [x] write handler tests using the existing minimock seam: golden
      no-filter (regression), filter subset (verify episodes only from
      selected seasons, `totalEpisodes` reflects subset, `defaultBest`
      recomputed, `availableSeasons` is full series), single-season filter,
      filter param malformed (400), filter intersecting to empty (400),
      filter that includes a non-existent season but at least one valid
      (uses the valid one, ignores the rest — or 400, decide and document
      in the test). Decision point: prefer "ignore non-existent, use the
      rest"; only 400 when the intersection is fully empty. Document this
      in code comment + OpenAPI.
- [x] run `make lint && make test-race` — must pass.

### Task 3: API contract + README

- [x] update `api/openapi.yaml`: add `seasons` query parameter to
      `/id/{id}` (string, pattern `^\d+(,\d+)*$`, optional, description
      explains filter + intersection rules); add `availableSeasons`
      property to `EpisodesResponse` schema (always present, integer array,
      ascending). `defaultBest` and `totalEpisodes` descriptions updated to
      note "over the selected subset when `seasons` is provided".
- [x] update `README.md`: extend the `/id/{id}` section with an example
      that uses `seasons=`, document the "absent = all" default, note that
      `defaultBest`/`totalEpisodes` reflect the subset, and that
      `availableSeasons` is the source of truth for selector UI.
- [x] no test changes here; validate by re-reading CLAUDE.md's "Doc
      discipline" rule — prose must be derived from the code written in
      Task 2.

### Task 4: Frontend — `useUrlState` gains `seasons`

- [x] extend `UrlState` in `frontend/src/hooks/useUrlState.ts` with
      `seasons: number[] | null`. `null` = "all selected" (omitted from
      URL). Parse `seasons=1,3,5`: split on comma, coerce to int, drop
      non-positive / NaN, dedupe, sort ascending; an empty list after
      cleanup → `null`. Serialize back as ascending CSV; omit when `null`.
- [x] update `useUrlState.test.ts` with cases for parse (valid CSV,
      reordered, duplicates, mixed garbage, empty), serialize (null →
      omitted, list → CSV), and round-trip via the existing pushState
      pattern.
- [x] run `npm run typecheck && npm run test:ci` — must pass.

### Task 5: Frontend — `getEpisodesById` accepts `seasons`

- [x] extend `getEpisodesById(id, language, limit?, seasons?, signal?)` in
      `frontend/src/lib/api.ts`. When `seasons` is a non-empty array, append
      `&seasons=1,3,5` (sorted ascending, deduped at call site or here —
      mirror what `useUrlState` already guarantees). When `seasons` is
      `null`/`undefined`/empty, omit the param.
- [x] add `availableSeasons: number[]` to `EpisodesResponse` in
      `frontend/src/lib/types.ts`.
- [x] update `api.test.ts` with URL-composition cases (no seasons, one,
      many; verify `&seasons=` ordering relative to existing `&limit=`).
- [x] run `npm run typecheck && npm run test:ci`.

### Task 6: Frontend — new `SeasonSelector` component

- [x] create `frontend/src/components/SeasonSelector.tsx`. Props:
      `available: number[]`, `selected: number[] | null` (null = all),
      `onChange: (next: number[] | null) => void`. Render an "All" chip
      followed by one chip per season number. Toggling a single chip
      switches state out of "all" into an explicit list (initial list =
      every season except the one being deselected); toggling "All" sets
      `null`. When the explicit list ends up containing every available
      season, the component should normalize back to `null` so the URL
      stays clean. Hidden when `available.length <= 1`.
- [x] mobile-friendly chip row: wrap on overflow, accessible
      `aria-pressed` state on each chip, keyboard activation via
      Space/Enter. Reuse Tailwind utility classes already in
      `EpisodesSlider.tsx` for visual consistency.
- [x] write `SeasonSelector.test.tsx`: renders chips for each available
      season, "All" pressed when `selected` is null, toggle-off one chip
      emits an array missing only that season, toggle-off the last chip in
      an explicit list emits `[]` (caller layer can decide; the
      `EpisodesList` layer forbids that — see Task 7), toggle "All" emits
      `null`, deselecting all-but-one and then re-selecting the last
      normalizes back to `null`.
- [x] run `npm run typecheck && npm run test:ci && npm run lint`.

### Task 7: Frontend — wire selector into `EpisodesList`

- [x] add props `selectedSeasons: number[] | null` and `onSeasonsChange:
      (next: number[] | null) => void` to `EpisodesList`.
- [x] include `selectedSeasons` (normalized to a stable cache-key string
      via `?.join(',') ?? 'all'`) in the React Query key. Pass through to
      `getEpisodesById`.
- [x] on `selectedSeasons` change: reset `fetchLimit` to `undefined` and
      `sliderValue` to `null` (let the new response's `defaultBest` drive
      the slider) — the monotone-in-N invariant the local re-slice
      depends on does not hold across season-set changes.
- [x] render `SeasonSelector` above `EpisodesSlider`, fed by
      `data.availableSeasons` and `selectedSeasons`. Forbid empty
      selection at this layer: if `onSeasonsChange` is called with `[]`,
      coerce to `null` (the API contract treats empty as "all") and let
      the URL drop the param.
- [x] update `EpisodesList.test.tsx`: refetch fires on selection change,
      slider+fetchLimit reset to defaults on change, episode list updates
      to reflect the filtered response, selector shows all seasons in
      `availableSeasons` regardless of filter.
- [x] run `npm run typecheck && npm run test:ci`.

### Task 8: Frontend — wire `seasons` through `App.tsx`

- [x] thread `seasons` from `useUrlState()` into `EpisodesList`'s new
      props; on series change (`onSelect`) and on "back to search" reset
      `seasons: null` alongside the existing `best: null` reset.
- [x] no new component test here; relies on the e2e test in Task 9 and
      existing App-level tests if any.
- [x] run `npm run typecheck && npm run lint && npm run test:ci`.

### Task 9: E2E smoke — toggle a season

- [x] extend the existing Playwright smoke under `frontend/e2e/`: open the
      same series the smoke test already uses, capture the original
      "X of Y" counter, toggle off one season chip, assert the counter's
      `Y` decreased and that no rendered episode has the toggled-off
      season number; toggle "All" and assert original `Y` returns.
- [x] run `npm run test:e2e` locally — must pass. (skipped — automated
      run requires `npx playwright install chromium` plus a live backend
      with TMDB key serving the SPA; covered by the Post-Completion
      manual verification flow. Test was syntactically validated via
      `npx playwright test --list` and `npx eslint`.)

### Task 10: Verify acceptance criteria

- [x] verify `seasons` absent → response is byte-equivalent to today
      modulo the new `availableSeasons` field (golden Go test from Task 2 —
      `TestIDHandler/default` matches `defaultBestBody` byte-for-byte
      including only the new `availableSeasons: [1,2,3]` field).
- [x] verify `defaultBest` recomputes over the subset (Go test from
      Task 2 — `seasonsSingle` and `seasonsSubset` cases prove
      `totalEpisodes` reflects the filtered subset and `defaultBest`
      flows from the subset through the same algorithm; algorithm-only
      coverage in `TestIDHandlerDefaultBestConfig`).
- [x] verify URL is omitted when all seasons selected (FE test from
      Task 4 — `useUrlState.test.ts` "omits seasons param when null or
      empty" passes).
- [x] verify slider resets on season-set change (FE test from Task 7 —
      `EpisodesList.test.tsx` "toggling a season triggers a refetch with
      the new seasons param and resets the slider" passes).
- [x] run full test suite: `make lint && make test-race && (cd frontend
      && npm run lint && npm run typecheck && npm run test:ci)` — all
      green (Go lint 0 issues; race tests pass across all packages;
      frontend lint clean; tsc clean; 80/80 vitest tests pass).
- [x] run `npm run test:e2e` (skipped — requires `npx playwright install
      chromium` and a live TMDB-backed backend serving the SPA, see
      Task 9; covered by Post-Completion manual flow).
- [x] verify `availableSeasons` is always full-series, even under a
      filter (Go: `TestIDHandlerSeasonsAvailableSeasonsAlwaysFull`; FE:
      `EpisodesList.test.tsx` "renders the SeasonSelector populated from
      availableSeasons regardless of filter").

### Task 11: Final — documentation pass

- [x] re-read `README.md` `/id/{id}` block against the final handler code
      to satisfy CLAUDE.md's "doc prose derived from code" rule. Fixed
      stale `defaultBest` prose on line 45 ("rating is at or above the
      series average" → "strictly exceeds the configured quantile of all
      ratings (default `0.9`), raised to a minimum-episode floor (default
      `3`) when fewer episodes clear the threshold") so it matches
      `computeDefaultBest` (`countAboveStrict` over `quantileRating`,
      floored by `DefaultBestMinEpisodes`) and the OpenAPI prose. The
      `seasons`, `availableSeasons`, intersection-rule, and 400 prose
      already matched the handler.
- [x] re-read `api/openapi.yaml` `/id/{id}` block against the final
      handler code. Already in sync: `seasons` parameter description
      (parsing, intersection, 400-on-empty), 400/500 response prose
      (malformed filter / empty intersection vs. invalid id, TMDB error,
      zero episodes), and the `EpisodesResp` schema (`defaultBest`,
      `totalEpisodes`, `availableSeasons` all marked required) match
      `idHandler` and `loadFilteredSeasons`.
- [x] no other doc files (CLAUDE.md, plans/) need updates.

*Note: ralphex automatically moves completed plans to `docs/plans/completed/`*

## Technical Details

**Filter shape**

- Wire format: `seasons=1,3,5` — comma-separated positive ints, server
  accepts any order with whitespace, but emits documentation as
  ascending-sorted unique.
- Backend representation: `map[int]struct{}` for the keep-set passed into
  the filter helper; `[]int` for `availableSeasons` and request parsing.
- "All seasons" sentinel: param absent / empty after parse → no filter
  (current behaviour). The frontend mirrors this with `seasons: null`.

**Empty-intersection rule**

- Filter contains *only* season numbers that don't exist for the series →
  400 (logged, like the existing 500 path for zero-episode series — but
  this is a client error, not a server one).
- Filter contains a mix of valid and invalid → use the valid intersection,
  ignore the rest. Document in code + OpenAPI. Rationale: lets a stable
  saved link survive a series losing a season in TMDB without the page
  blowing up.

**Cache interaction**

- `tvmeta.TVShowAllSeasonsWithDetails(id, language)` cache key is
  unchanged (series-wide, post-IMDb-override). The handler filters the
  cached read-only `*AllSeasonsWithDetails`; `flattenSortedByRating` already
  allocates a new `[]episode`, so no mutation risk.
- No new cache. A keyed-by-filter cache would explode the key space (one
  entry per subset of seasons) for negligible win — the post-cache filter
  is O(episodes) and runs in microseconds.

**Frontend monotone-in-N invariant**

- The re-slice optimization in `EpisodesList` ("a larger response always
  contains every episode of a smaller one") holds *within a fixed season
  set*. Across selection changes it breaks: a smaller-N response after a
  selection change does not contain the same episodes. Resetting
  `fetchLimit` to `undefined` and `sliderValue` to `null` on selection
  change forces a clean reload at the new server-default position.

**URL serialization**

- `seasons` lives alongside `q,id,lang,best` in `useUrlState`. Omitted
  when `null` (all selected). Sorted ascending. Reset to `null` on series
  change and on "back to search" (matches the existing `best: null` reset).

## Post-Completion

*Items requiring manual intervention or external systems — no checkboxes,
informational only.*

**Manual verification (per CLAUDE.md "Before claiming a task done")**

- `make docker-build && docker compose up -d`
- `until curl -fsS http://127.0.0.1:8202/ping >/dev/null; do sleep 1; done`
- Hit:
  - `curl -fsS 'http://127.0.0.1:8202/id/1399?language=en' | jq .` — verify
    `availableSeasons` present, `seasons` field unused.
  - `curl -fsS 'http://127.0.0.1:8202/id/1399?language=en&seasons=1,2' |
    jq '{totalEpisodes, defaultBest, available: .availableSeasons,
    seasons: [.episodes[].season] | unique}'` — verify subset semantics.
  - `curl -fsS 'http://127.0.0.1:8202/id/1399?language=en&seasons=999' -o
    /dev/null -w '%{http_code}\n'` — verify 400.
  - `curl -fsS 'http://127.0.0.1:8202/id/1399?language=en&seasons=abc' -o
    /dev/null -w '%{http_code}\n'` — verify 400.
- Browser walkthrough (per CLAUDE.md "Frontend verification gotchas"):
  open the SPA against the running container, set Chrome UA to iOS
  Safari, validate at iPhone widths (375 and 390 px) by reading
  `getBoundingClientRect()` on the chip row to confirm wrap and tap
  targets. Toggle a season, watch the slider's `Y` counter drop, share
  the URL, paste in a new tab, verify the same filter loads.
- If a UI change isn't visible after `docker compose up -d`, suspect the
  service worker cache first (per CLAUDE.md): unregister + clear caches
  in DevTools, hard reload.
- `docker compose down` when finished.

**External system updates**

- None. The change is self-contained: no consuming projects, no
  deployment-config touch, no third-party integrations.
