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

## Testing notes

- Mocks live under `*/mocks/` and are minimock-generated. The `//go:generate` directive is in the mock file itself, so adding a new interface means adding a stub mock file with the directive *or* running minimock manually once, then `make generate-mocks` keeps it in sync.
- `golangci-lint` is configured with `tests: false`, so test files don't go through the linter — don't be surprised that lint passes on test code with patterns the production linter would reject.
- The frontend has one Playwright smoke test (`frontend/e2e/`) that runs against `vite preview` or the Docker container; unit/component tests use Vitest + RTL.
