# Lazy Soap — frontend

Single-page web client for the [Lazy Soap](../README.md) backend. Lets you
search for a TV series and shows the best episodes (above-average rated).

## Stack

- **Vite** + **React 19** + **TypeScript (strict)**
- **Tailwind CSS v4** for styling
- **TanStack Query v5** for data fetching, caching, and request cancellation
- **vite-plugin-pwa** for offline app shell + installable manifest
- **Vitest** + **React Testing Library** for unit/component tests
- **Playwright** for one end-to-end smoke test
- **ESLint** (typescript-eslint, react-hooks, jsx-a11y) + **Prettier**

The production build is a static SPA: `npm run build` writes to `frontend/build/`,
which the multi-stage `Dockerfile` copies into the Go server's `/static/`. The
backend (`internal/pkg/rest/static.go`) serves it on the same origin as the
JSON API (`/search/...`, `/id/...`, `/img/...`).

## Local development

Prerequisites: Node ≥ 22 (LTS) and npm. Run the backend separately, e.g.

```bash
make docker-build && make docker-up   # serves on http://127.0.0.1:8202
```

Then in another shell:

```bash
cd frontend
npm ci
npm run dev                            # http://localhost:5173
```

The dev server proxies `/search`, `/id`, `/img`, `/ping`, `/version` to
`http://127.0.0.1:8202`. To talk to a different backend instead, copy
`.env.development.example` to `.env.development.local` and set
`VITE_API_BASE_URL` to an absolute URL.

## Scripts

| Command                | What it does                              |
| ---------------------- | ----------------------------------------- |
| `npm run dev`          | Vite dev server with HMR                  |
| `npm run build`        | Type-check + production build to `build/` |
| `npm run preview`      | Serve `build/` locally for manual checks  |
| `npm run lint`         | ESLint over `src/`                        |
| `npm run typecheck`    | `tsc -b --noEmit`                         |
| `npm run test`         | Vitest watch mode                         |
| `npm run test:ci`      | Vitest one-shot (CI)                      |
| `npm run test:e2e`     | Playwright smoke against `preview`/Docker |
| `npm run format`       | Prettier write                            |
| `npm run format:check` | Prettier check                            |

## Layout

```
src/
  main.tsx, App.tsx, index.css
  lib/        api.ts, types.ts, poster.ts, queryClient.ts
  hooks/      useDebouncedValue, useKeyboardListNav
  components/ Header, SeriesCombobox, SearchResultRow, SelectedSeriesCard,
              EpisodesList, EpisodeRow, Spinner, EmptyState, ErrorState,
              ErrorBoundary
```
