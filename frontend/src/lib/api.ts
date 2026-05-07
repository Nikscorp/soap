import type {
  EpisodesResponse,
  FeaturedResponse,
  MetaResponse,
  SearchResponse,
} from './types';

// Base URL for backend calls. Empty string ("") means same-origin, which is
// what production uses since the Go server hosts both the SPA and the API.
// During `npm run dev`, vite.config.ts proxies /search, /id, /img to the local
// backend, so leaving this empty still works. Override via
// VITE_API_BASE_URL=https://soap.example.com for talking to a remote backend.
const BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

async function request<T>(path: string, signal?: AbortSignal): Promise<T> {
  const url = `${BASE_URL}${path}`;
  const res = await fetch(url, {
    method: 'GET',
    headers: { Accept: 'application/json' },
    signal,
  });
  if (!res.ok) {
    throw new ApiError(`Request to ${path} failed`, res.status);
  }
  return (await res.json()) as T;
}

export function searchSeries(query: string, signal?: AbortSignal): Promise<SearchResponse> {
  return request<SearchResponse>(`/search/${encodeURIComponent(query)}`, signal);
}

export function getEpisodesById(
  id: number,
  language: string,
  limit?: number,
  seasons?: number[] | null,
  signal?: AbortSignal,
): Promise<EpisodesResponse> {
  const lang = encodeURIComponent(language || 'en');
  const limitQs = limit && limit > 0 ? `&limit=${encodeURIComponent(String(limit))}` : '';
  const seasonsQs = buildSeasonsQs(seasons);
  return request<EpisodesResponse>(
    `/id/${encodeURIComponent(String(id))}?language=${lang}${limitQs}${seasonsQs}`,
    signal,
  );
}

function buildSeasonsQs(seasons: number[] | null | undefined): string {
  if (!seasons || seasons.length === 0) return '';
  const cleaned = Array.from(
    new Set(
      seasons
        .filter((n) => Number.isFinite(n) && n > 0)
        .map((n) => Math.trunc(n)),
    ),
  ).sort((a, b) => a - b);
  if (cleaned.length === 0) return '';
  return `&seasons=${cleaned.join(',')}`;
}

export function getFeaturedSeries(
  language: string,
  signal?: AbortSignal,
): Promise<FeaturedResponse> {
  const lang = encodeURIComponent(language || 'en');
  return request<FeaturedResponse>(`/featured?language=${lang}`, signal);
}

// getMeta returns server-wide metadata such as the configured rating source.
// The backend caches the response with `Cache-Control: public, max-age=300`
// so React Query's default staleness still allows a fresh check after restart.
export function getMeta(signal?: AbortSignal): Promise<MetaResponse> {
  return request<MetaResponse>(`/meta`, signal);
}

// Poster path normalization. The backend currently returns relative paths
// like "/img/<file>.jpg" served by its own proxy handler; if it ever returns
// absolute http(s) URLs (e.g. straight from TMDB), we pass them through.
// A bare "<file>.jpg" is treated as a poster path under /img/.
//
// `size` lets callers request a larger TMDB variant (allow-list lives in the
// backend: w92, w154, w185, w342, w500, w780). Absolute URLs are returned
// untouched since their size is already baked in.
export function normalizePosterUrl(
  poster: string | null | undefined,
  size?: string,
): string | null {
  if (!poster) return null;
  if (/^https?:\/\//i.test(poster)) return poster;
  const path = poster.startsWith('/') ? poster : `/img/${poster}`;
  const qs = size ? `?size=${encodeURIComponent(size)}` : '';
  return `${BASE_URL}${path}${qs}`;
}

// Poster sizes proxied through /img. Mirrors the backend allow-list in
// internal/pkg/tvmeta/poster.go; pinning here keeps the SPA from asking for
// a size the server would coerce away.
export type Size = 'w185' | 'w342' | 'w500' | 'w780';

// posterSrcSet builds a width-descriptor srcset for the /img proxy. Pairs
// each requested rendition with its TMDB pixel-width so the browser can
// honor the matching `sizes` attribute and pick the smallest variant that
// still satisfies layout × DPR. Absolute http(s) poster URLs (rare; only if
// TMDB ever leaks past the proxy) yield an empty string — width descriptors
// don't apply when the URL is opaque.
export function posterSrcSet(
  poster: string | null | undefined,
  sizes: readonly Size[],
): string {
  if (!poster || sizes.length === 0) return '';
  if (/^https?:\/\//i.test(poster)) return '';
  return sizes
    .map((size) => {
      const url = normalizePosterUrl(poster, size);
      if (!url) return '';
      const width = Number(size.slice(1));
      return `${url} ${width}w`;
    })
    .filter((s) => s !== '')
    .join(', ');
}
