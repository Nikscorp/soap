# Lazy Soap

<div align="center">

[![Coverage Status](https://coveralls.io/repos/github/Nikscorp/soap/badge.svg?branch=master)](https://coveralls.io/github/Nikscorp/soap?branch=master)&nbsp;[![Build Status](https://github.com/Nikscorp/soap/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/Nikscorp/soap/actions)&nbsp;[![Go Report Card](https://goreportcard.com/badge/github.com/Nikscorp/soap)](https://goreportcard.com/report/github.com/Nikscorp/soap)&nbsp;[![Health Status](https://gatus.nivoynov.dev/api/v1/endpoints/_soap-ping/health/badge.svg)](https://gatus.nivoynov.dev/endpoints/_soap-ping)

</div>

A small website that surfaces the highest-rated episodes of a TV series, backed by [TMDB](https://www.themoviedb.org/) metadata. Useful for anthology shows like *Black Mirror* where episodes stand alone, or for revisiting the peaks of a long-running series without rewatching the whole thing.

## Usage

A public instance lives at [https://soap.nivoynov.dev](https://soap.nivoynov.dev). You can also self-host by pulling the pre-built image, building one locally, or running the binary directly — see [Installation](#installation).

The web client and the JSON API are served from the same origin. The frontend is a static SPA bundled into the Docker image; the Go server hosts both the API and the SPA assets.

## API

A full OpenAPI 3 spec lives at [`api/openapi.yaml`](api/openapi.yaml). Four endpoints back the app:

### `GET /search/{query}` — search series by name

```json
{
  "searchResults": [
    {
      "id": 42009,
      "title": "Black Mirror",
      "firstAirDate": "2011-12-04",
      "poster": "/img/7PRddO7z7mcPi21nZTCMGShAyy1.jpg",
      "rating": 8.3,
      "description": "A British anthology television series that examines modern society."
    }
  ],
  "language": "en"
}
```

Posters are returned as relative `/img/{path}` references that resolve through the proxy below — the same image is reachable at multiple sizes via `?size=`.

### `GET /id/{id}?language=&limit=` — best episodes for a series

`language` is an optional ISO 639-1 code forwarded to TMDB. `limit` is an optional positive integer that caps the response to the top-`limit` episodes by rating. When `limit` is omitted (or invalid), the response falls back to `defaultBest` — the count of episodes whose rating is at or above the series average. The returned slice is selected by rating but ordered chronologically by `(season, number)` for display. Episodes carry an optional `still` (a `/img/{path}` reference to the TMDB still frame). `description` is the series synopsis from TMDB (localized via `language`); it can be an empty string when TMDB has no overview for the series in the requested language.

```json
{
  "title": "Black Mirror",
  "poster": "/img/7PRddO7z7mcPi21nZTCMGShAyy1.jpg",
  "firstAirDate": "2011-12-04",
  "description": "A British anthology television series that examines modern society, particularly with regard to the unanticipated consequences of new technologies.",
  "defaultBest": 3,
  "totalEpisodes": 27,
  "episodes": [
    {
      "title": "The National Anthem",
      "description": "The British prime minister faces a shocking dilemma when Princess Susannah is kidnapped.",
      "rating": 7.504,
      "number": 1,
      "season": 1,
      "still": "/img/lj0R2Lo3oqxiSJp9XwVKr6IRPKp.jpg"
    }
  ]
}
```

### `GET /featured?language=` — random featured series for the home screen

Returns `LAZYSOAP_FEATURED_COUNT` (default 3) randomly chosen series, deduplicated by id, drawn from the union of:

- TMDB's "popular TV" endpoint (page 1), filtered to entries with `vote_count >= LAZYSOAP_FEATURED_MIN_VOTE_COUNT` (default 100) so unrated/spam entries are dropped.
- An operator-curated list of TMDB ids (`LAZYSOAP_FEATURED_EXTRA_IDS`). These bypass the vote-count filter — if it's in your list, it's trusted.

Each request reshuffles, so refreshing the home page surfaces different shows. The response sets `Cache-Control: no-store`. Operator extras are warmed into an in-memory cache at startup and refreshed every `LAZYSOAP_FEATURED_EXTRAS_REFRESH_INTERVAL` (default 24h) — the request path itself never round-trips TMDB for those static ids. If the unioned pool would have fewer than `FEATURED_COUNT` items the handler returns 503 instead of a short list.

```json
{
  "series": [
    {
      "id": 42009,
      "title": "Black Mirror",
      "firstAirDate": "2011-12-04",
      "poster": "/img/7PRddO7z7mcPi21nZTCMGShAyy1.jpg"
    },
    {
      "id": 1399,
      "title": "Game of Thrones",
      "firstAirDate": "2011-04-17",
      "poster": "/img/1XS1oqL89opfnbLl8WnZY1O1uJx.jpg"
    },
    {
      "id": 66732,
      "title": "Stranger Things",
      "firstAirDate": "2016-07-15",
      "poster": "/img/49WJfeN0moxb9IPfGn8AIqMGskD.jpg"
    }
  ],
  "language": "en"
}
```

### `GET /img/{path}?size=` — TMDB poster proxy

Streams the TMDB poster identified by the trailing filename of a TMDB image URL (for example `7PRddO7z7mcPi21nZTCMGShAyy1.jpg`). Lets the SPA load posters through the same origin without exposing TMDB directly. The optional `size` query parameter selects a TMDB image variant from the allow-list `w92, w154, w185, w342, w500, w780`; anything else is coerced back to the default (`w92`).

### Operational endpoints

`GET /ping` returns `pong`, `GET /version` returns the build version, `GET /metrics` exposes Prometheus metrics, and `GET /debug/pprof/*` exposes the Go pprof handlers.

## Installation

To self-host you need a TMDB API key (free; obtain it from [TMDB settings](https://www.themoviedb.org/settings/api)).

### Run the pre-built image

1. Copy `docker-compose.yml` (or `docker-compose-remote.yml` if you want the published `ghcr.io/nikscorp/soap:latest` image) and `config/config.yaml.dist` into a working directory and rename the config to `config/config.yaml`.
2. Set your TMDB API key. Either edit `tmdb.api_key` in `config/config.yaml`, or override at runtime via the `TMDB_API_KEY` environment variable.
3. Tweak the published port, log size, or timeouts in `docker-compose.yml` if needed (the default binds to `127.0.0.1:8202`).
4. Pull and start the service:
   ```sh
   docker compose pull
   docker compose up -d
   ```
5. Put it behind an HTTPS reverse proxy if you intend to expose it publicly.

### Build a custom image

Same as above but replace `docker compose pull` with `docker compose build` to build the image locally from this repository.

### Build from source

The multi-stage `Dockerfile` is the source of truth. In short:

```sh
# Backend (Go 1.26+)
make build           # writes ./bin/lazysoap

# Frontend (Node 22+)
cd frontend && npm ci && npm run build   # writes ./frontend/build

# Run, pointing the binary at a config file. The frontend assets must
# live at /static/ — the Dockerfile mirrors that layout; in development
# run the Vite dev server from `frontend/` (see frontend/README.md).
./bin/lazysoap -c config/config.yaml
```

## Configuration

Defaults live in `config/config.yaml.dist`. Every field can also be overridden through environment variables (handled by [cleanenv](https://github.com/ilyakaznacheev/cleanenv)). The most useful ones:

| Variable | Default | Purpose |
| --- | --- | --- |
| `TMDB_API_KEY` | *(required)* | TMDB API key |
| `LAZYSOAP_LISTEN_ADDR` | `0.0.0.0:8080` | HTTP listen address |
| `LAZYSOAP_READ_TIMEOUT` / `LAZYSOAP_WRITE_TIMEOUT` / `LAZYSOAP_IDLE_TIMEOUT` | `10s` / `10s` / `10s` | Server timeouts |
| `LAZYSOAP_GRACEFUL_TIMEOUT` | `10s` | Shutdown grace period |
| `LAZYSOAP_FEATURED_COUNT` | `3` | Number of series returned by `/featured` |
| `LAZYSOAP_FEATURED_MIN_VOTE_COUNT` | `100` | Min `vote_count` an entry from TMDB's popular pool must have to be eligible. Curated extras bypass this. |
| `LAZYSOAP_FEATURED_EXTRA_IDS` | curated TMDB ids | Comma-separated TMDB ids always eligible for `/featured` (e.g. `1399,1396,1668`). |
| `LAZYSOAP_FEATURED_EXTRAS_REFRESH_INTERVAL` | `24h` | How often the extras cache is refreshed; `0` means startup-only. |
| `TMDB_REQUEST_TIMEOUT` | `10s` | Outbound TMDB request timeout |
| `TMDB_ENABLE_AUTO_RETRY` | `true` | Retry transient TMDB errors |

See `internal/app/lazysoap/server.go` and `internal/pkg/clients/tmdb/tmdb.go` for the full set of fields and YAML keys.

## Development

Common workflows are exposed through `make`:

| Target | Description |
| --- | --- |
| `make build` | Build the Go binary into `bin/lazysoap` |
| `make lint` | `golangci-lint run ./...` |
| `make test` / `make test-race` / `make test-cov` | Run unit tests; race detector; coverage with HTML report |
| `make generate-mocks` | Regenerate [minimock](https://github.com/gojuno/minimock) mocks |
| `make tidy` | `go mod tidy && go mod vendor` |
| `make docker-build` / `make docker-up` | Build/run the full stack with Docker Compose |

The Go side uses [chi](https://github.com/go-chi/chi) for routing, [cleanenv](https://github.com/ilyakaznacheev/cleanenv) for config, and [prometheus/client_golang](https://github.com/prometheus/client_golang) for metrics. Frontend stack and scripts are documented in [`frontend/README.md`](frontend/README.md) — React 19, Vite, TypeScript, Tailwind v4, TanStack Query, and a Playwright smoke test.

## License

[MIT](LICENSE)
