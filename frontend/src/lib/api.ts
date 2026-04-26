import type { EpisodesResponse, SearchResponse } from './types';

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
  signal?: AbortSignal,
): Promise<EpisodesResponse> {
  const lang = encodeURIComponent(language || 'en');
  const limitQs = limit && limit > 0 ? `&limit=${encodeURIComponent(String(limit))}` : '';
  return request<EpisodesResponse>(
    `/id/${encodeURIComponent(String(id))}?language=${lang}${limitQs}`,
    signal,
  );
}

// Poster path normalization. The backend currently returns relative paths
// like "/img/<file>.jpg" served by its own proxy handler; if it ever returns
// absolute http(s) URLs (e.g. straight from TMDB), we pass them through.
// A bare "<file>.jpg" is treated as a poster path under /img/.
export function normalizePosterUrl(poster: string | null | undefined): string | null {
  if (!poster) return null;
  if (/^https?:\/\//i.test(poster)) return poster;
  if (poster.startsWith('/')) return `${BASE_URL}${poster}`;
  return `${BASE_URL}/img/${poster}`;
}
