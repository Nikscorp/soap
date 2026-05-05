# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository shape

LazySoap is a Go HTTP server that wraps TMDB, paired with a React/Vite SPA. In production both ship as a single image: the Go binary serves the JSON API *and* the SPA bundle from the same origin. The frontend is built to `frontend/build/` and copied into the image at `/static/`, which `internal/pkg/rest/static.go` serves at `/`. There is no separate frontend host — keep that in mind when reasoning about CORS, paths, and caching.

## Common commands

Backend (Go ≥ 1.26, run from repo root):

| Command | Purpose |
| --- | --- |
| `make build` | Builds `bin/lazysoap` with `-X main.version=…` injected from git |
| `make lint` | `golangci-lint run ./...` — config in `.golangci.yml` (`default: all` minus a curated disable list; tests are NOT linted; `vendor` and `*/mocks/*` excluded) |
| `make test` / `make test-race` / `make test-cov` | Plain / race / coverage (writes `bin/cover.out` and opens HTML) |
| `make generate-mocks` | Regenerates minimock mocks (driven by `//go:generate` lines inside each mock file — see `internal/app/lazysoap/mocks/` and `internal/pkg/tvmeta/mocks/`) |
| `make tidy` | `go mod tidy && go mod vendor` (the repo vendors deps) |
| `make docker-build` / `make docker-up` | Full stack via `docker-compose.yml` (default bind `127.0.0.1:8202`) |

Run a single Go test: `go test -run TestName$ ./internal/path/...` (use `-race` when touching concurrent code — the featured-extras cache and `TVShowAllSeasonsWithDetails` errgroup are the hot spots).

**Docker builds: foreground or `Monitor`, never `sleep` polling.** `make docker-build` takes 60–120s. Run it foreground with `timeout: 180000` on the Bash call, or background it and use the `Monitor` tool to await completion. Never chain `sleep N && tail` — the harness blocks long sleeps and you'll burn a turn on the rejection.

Frontend (Node ≥ 22, run from `frontend/`):

| Command | Purpose |
| --- | --- |
| `npm run dev` | Vite dev on `:5173`, proxies `/search`, `/id`, `/img`, `/ping`, `/version` to `http://127.0.0.1:8202` (the Docker bind). Override with `VITE_API_BASE_URL` in `.env.development.local`. |
| `npm run build` | `tsc -b && vite build` → `frontend/build/` (this is what the Dockerfile copies) |
| `npm run lint` / `typecheck` / `test` / `test:ci` / `test:e2e` | ESLint / `tsc -b --noEmit` / Vitest watch / Vitest one-shot / Playwright smoke |

Local dev typically means running the backend in Docker (`make docker-up`) on `:8202` while running `npm run dev` for the SPA.

## Architecture

Wiring (top of `cmd/lazysoap/main.go`):

```
config.ParseConfig  →  tmdb.NewTMDB(cfg.TMDBConfig)  →  tvmeta.New(tmdbClient)  →  lazysoap.New(cfg, tvMeta, version).Run(ctx)
```

Three layers, in dependency order:

1. **`internal/pkg/clients/tmdb`** — thin auth/retry wrapper around `github.com/cyruzin/golang-tmdb`. Knows nothing about our domain types.
2. **`internal/pkg/tvmeta`** — domain layer that hides TMDB quirks. Defines a `tmdbClient` interface (the seam used by tests/mocks) and exposes `SearchTVShows`, `TVShowDetails`, `TVShowAllSeasonsWithDetails`, `PopularTVShows`. `TVShowAllSeasonsWithDetails` fans out one goroutine per season via `errgroup` — preserve that when extending it.
3. **`internal/app/lazysoap`** — HTTP layer. `server.go` builds the chi router and declares the `tvMetaClient` interface it consumes (this is the mock seam for handler tests). One file per endpoint: `search.go`, `by_id.go`, `featured.go`, `img_proxy.go`. `rest.AddFileServer` mounts the SPA last as the catch-all.

Two pieces of non-obvious behavior to keep in mind when editing:

- **Featured-extras cache (`featured.go`).** Curated TMDB IDs are resolved at startup and refreshed by a background goroutine (`runFeaturedExtrasRefresh`, kicked off from `Server.Run`). Reads use `atomic.Pointer[[]featuredItem]` — copy-on-write swap, no locks on the request path. The cached slice is shared and **read-only**; never mutate or append to a slice returned by `featuredExtras.view()`. The refresh goroutine is started async on purpose so a slow/down TMDB at boot can't block `ListenAndServe` (k8s liveness, fast restarts).
- **Static asset cache headers (`internal/pkg/rest/static.go`).** Vite content-hashed assets under `/assets/` and Workbox runtime get `max-age=31536000, immutable`; the SPA shell, manifest, and main service worker are `no-cache` so deploys propagate immediately. If you add new top-level static files, decide which bucket they fall into.

## Configuration

`internal/pkg/config/config.go` calls `cleanenv.ReadConfig`, so every field has a YAML key *and* an env var (`env-default` provides the fallback). Defaults live in `config/config.yaml.dist`; the runtime config is `config/config.yaml`. The two struct trees worth knowing:

- `internal/app/lazysoap/server.go` — `Config` (server timeouts, `LAZYSOAP_FEATURED_*`, `DefaultBestQuantile`/`DefaultBestMinEpisodes` for the `/id/{id}` "default best" computation, `ImgClient` for the poster proxy).
- `internal/pkg/clients/tmdb/tmdb.go` — `Config` (`TMDB_API_KEY` is required; `TMDB_REQUEST_TIMEOUT`, `TMDB_ENABLE_AUTO_RETRY`).

When adding a new tunable, add it to both the struct (with `env`, `env-default`, `yaml` tags) and to `config/config.yaml.dist`.

## Public API contract

Four endpoints, fully specified in `api/openapi.yaml` (treat that file as the contract — update it when handler shapes change):

- `GET /search/{query}` — series search.
- `GET /id/{id}?language=&limit=` — best episodes. `defaultBest` is server-computed (top quantile, floored at `DefaultBestMinEpisodes`); episodes are *selected* by rating but *returned* in chronological `(season, number)` order.
- `GET /featured?language=` — randomized pool drawn from TMDB popular ∪ curated extras; returns 503 (not a short list) if the pool can't satisfy `FeaturedCount`. Always `Cache-Control: no-store`.
- `GET /img/{path}?size=` — TMDB poster proxy. `size` is allow-listed to `w92,w154,w185,w342,w500,w780`; anything else is silently coerced to `w92`. Posters in JSON responses are emitted as relative `/img/{path}` references that resolve through this handler — don't leak raw TMDB URLs to the client.

Operational: `/ping`, `/version`, `/metrics` (Prometheus), `/debug/pprof/*`.

## Before claiming a task done

For any change that touches the public API contract (Go handlers, response shapes) or user-visible UI, do **all** of the following before reporting completion. Tests and lint passing is necessary but not sufficient. Pure frontend layout/styling changes (no Go file or `api/openapi.yaml` touched) skip steps 1–2 but never skip 3–4. State the scope explicitly in the completion report so the user can audit.

1. **README.md.** It contains a hand-written API tour with JSON examples for `/search/{query}`, `/id/{id}`, `/featured`, `/img/{path}`. Whenever you add/remove/rename a field on any of those, or change defaults/behavior described in prose (e.g. `defaultBest` semantics, featured pool rules, image size allow-list), update the matching section. The OpenAPI spec is the contract; the README is the human-facing summary — both must move together.
2. **`api/openapi.yaml`.** Keep schemas exhaustive. New optional fields go into `properties` but stay out of `required` unless the server *always* emits them.
3. **End-to-end via Docker.** Type checks and unit tests confirm code-level correctness; they do not exercise the integrated stack (real TMDB calls, frontend bundle copied into the image, static asset serving). Run:
   ```sh
   make docker-build
   docker compose up -d
   until curl -fsS http://127.0.0.1:8202/ping >/dev/null; do sleep 1; done
   # exercise the changed endpoint(s) — at minimum the happy path,
   # plus localization (?language=ru) for anything TMDB-localized
   curl -fsS 'http://127.0.0.1:8202/id/42009?language=en' | jq .
   docker compose down
   ```
   The image is multi-stage (frontend build → Go build → final), so a passing `make docker-build` also certifies that `npm run build` succeeded and the SPA bundle landed at `/static/`. Don't skip the `compose up` step — it's the only thing that exercises the bundled image against live TMDB.
4. **Frontend feature in a real browser** when UI changed. `npm run typecheck`/`test:ci` will not catch a broken layout, a missing prop, or an unstyled component. Either run `npm run dev` against `make docker-up` or open the Docker container's served SPA, navigate to the affected screen, and confirm visually.

## Documentation discipline

These two patterns silently cost time in past sessions; both are about the README, OpenAPI, and code-comment prose drifting away from what the code actually does.

- **README/OpenAPI/comment prose describing behavior must be derived from the code, not from the plan.** When you describe a cache, fallback, retry, freshness check, env-var precedence, or any rule in prose, generate that prose by reading the implementation. Past failure: the IMDb integration shipped a README claiming "container restarts don't re-download" while the actual `refreshFile` always called `download()` first and only fell back to the cached file on HTTP failure. The plan had said the right thing; the code didn't follow the plan; the README followed the plan, not the code. If you spot a plan/code disagreement, the doc tracks the code — or fix the code. Re-read the relevant function before writing the prose, every time.
- **Performance / size / latency numbers come from measurement, not estimation.** Memory, RSS, response time, file size, and cardinality figures in comments, README, OpenAPI descriptions, or completion reports need a real number from a real run, not "should be about ~". Past failures from this session alone: shipped "~85 MB resident heap" for the IMDb dataset (actual RSS ~290 MB); claimed "`initialTitlesCapacity = 1.5M` is too small, rehashes during build" with zero data — measurement showed 1.5M was already enough and bumping to 2M doubled the bucket count for no gain. If you must ship the doc before the box exists, leave a `TODO: measure` and confirm via `docker stats` and `/debug/pprof/heap?gc=1` once the container is up. If you can already run the box, do it before writing the number — `pprof -top -inuse_space -unit=mb` is a one-liner.

## Frontend verification gotchas

These caused multiple "I said done, user said it's still broken" cycles in past sessions. Internalize them.

- **Service worker, not Tailwind.** When a UI change doesn't appear after `docker compose up -d`, the cause is the SPA's service worker serving a stale shell — *not* Tailwind caching, *not* the build. Before blaming the build or doing `docker buildx build --no-cache`, verify by curling `http://127.0.0.1:8202/` for the `index-*.js` hash and `docker exec soap grep` the served CSS in `/static/assets/` for the new class. If both contain the new code, the issue is purely client-side cache. Tell the user to hard-reload, or to run in DevTools: `navigator.serviceWorker.getRegistrations().then(rs => Promise.all(rs.map(r => r.unregister()))); caches.keys().then(ks => Promise.all(ks.map(k => caches.delete(k))));` then reload.
- **macOS Chrome clamps window width to ~500px.** Resizing to 375 or 390 silently lands on ~500 innerWidth, so a real iPhone bug shows up milder (or not at all) in a desktop Chrome window. To validate iPhone widths, set the User-Agent to iOS Safari **and** read column sizes via `getBoundingClientRect()` on the actual element — don't eyeball screenshots. The user is on iOS Safari; match that.
- **Report numeric before/after for layout fixes.** A fix for "looks bad on mobile" must include a concrete delta the user can audit (e.g., "description column 80px → 301px at 375px width"). Vague "looks great now" claims have repeatedly turned out wrong on the user's actual device.

## Testing notes

- Mocks live under `*/mocks/` and are minimock-generated. The `//go:generate` directive is in the mock file itself, so adding a new interface means adding a stub mock file with the directive *or* running minimock manually once, then `make generate-mocks` keeps it in sync. **Run minimock from inside the target `mocks/` directory** (`cd internal/pkg/.../mocks && minimock -i …`). Running from the repo root bakes a path like `./internal/pkg/.../mocks/foo_mock.go` into the directive — relative to the wrong directory — and the next `make generate-mocks` regenerates the file in the wrong place. The existing `tmdb_client_mock.go` is the reference for the correct directive shape (`-o ./tmdb_client_mock.go`).
- `golangci-lint` is configured with `tests: false`, so test files don't go through the linter — don't be surprised that lint passes on test code with patterns the production linter would reject.
- The frontend has one Playwright smoke test (`frontend/e2e/`) that runs against `vite preview` or the Docker container; unit/component tests use Vitest + RTL.
- After an `Edit`, don't grep the file to "verify" the edit landed — the tool already errors on a miss. Re-read only when the next edit needs current line numbers.
- Bench convention for `internal/pkg/imdbratings`: `make bench` runs the always-on synthetic suite; `make bench-real` runs the `//go:build imdbbench`-gated real-data suite that reads the live gzipped TSVs from `LAZYSOAP_BENCH_DATA_DIR` (default `var/imdb/`). Pair with `make bench-baseline` / `make bench-real-baseline` and `benchstat` (developer-machine `go install golang.org/x/perf/cmd/benchstat@latest`) to compare runs.
