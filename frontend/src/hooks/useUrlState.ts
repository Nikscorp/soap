import { useCallback, useEffect, useState } from 'react';

export interface UrlState {
  q: string;
  id: number | null;
  lang: string;
  best: number | null;
  seasons: number[] | null;
}

export interface UseUrlState extends UrlState {
  setUrlState: (next: Partial<UrlState>, options?: { replace?: boolean }) => void;
}

const EMPTY: UrlState = { q: '', id: null, lang: '', best: null, seasons: null };

function readState(): UrlState {
  if (typeof window === 'undefined') return EMPTY;
  const params = new URLSearchParams(window.location.search);
  return {
    q: (params.get('q') ?? '').trim(),
    id: parsePositiveInt(params.get('id')),
    lang: (params.get('lang') ?? '').trim(),
    best: parsePositiveInt(params.get('best')),
    seasons: parseSeasons(params.get('seasons')),
  };
}

function parsePositiveInt(raw: string | null): number | null {
  if (raw == null || raw === '') return null;
  const n = Number.parseInt(raw, 10);
  return Number.isFinite(n) && n > 0 ? n : null;
}

function parseSeasons(raw: string | null): number[] | null {
  if (raw == null || raw === '') return null;
  const seen = new Set<number>();
  for (const tok of raw.split(',')) {
    const trimmed = tok.trim();
    if (trimmed === '') continue;
    const n = Number.parseInt(trimmed, 10);
    if (!Number.isFinite(n) || n <= 0) continue;
    seen.add(n);
  }
  if (seen.size === 0) return null;
  return Array.from(seen).sort((a, b) => a - b);
}

function normalizeSeasons(value: number[] | null): number[] | null {
  if (value === null) return null;
  const seen = new Set<number>();
  for (const n of value) {
    if (!Number.isFinite(n) || n <= 0) continue;
    seen.add(Math.trunc(n));
  }
  if (seen.size === 0) return null;
  return Array.from(seen).sort((a, b) => a - b);
}

function writeParam(params: URLSearchParams, key: string, value: string | number | null) {
  if (value === null || value === '' || (typeof value === 'number' && !Number.isFinite(value))) {
    params.delete(key);
  } else {
    params.set(key, String(value));
  }
}

function writeSeasonsParam(params: URLSearchParams, value: number[] | null) {
  if (value === null || value.length === 0) {
    params.delete('seasons');
  } else {
    params.set('seasons', value.join(','));
  }
}

function arraysEqual(a: number[] | null, b: number[] | null): boolean {
  if (a === b) return true;
  if (a === null || b === null) return false;
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    if (a[i] !== b[i]) return false;
  }
  return true;
}

function statesEqual(a: UrlState, b: UrlState): boolean {
  return (
    a.q === b.q &&
    a.id === b.id &&
    a.lang === b.lang &&
    a.best === b.best &&
    arraysEqual(a.seasons, b.seasons)
  );
}

// useUrlState binds the canonical URL params (q, id, lang, best, seasons) to
// React state. It mirrors the pushState/replaceState/popstate pattern of the
// previous useUrlQuery hook but supports atomic multi-param updates so
// transitions (search → episodes, slider drag, back/forward) stay race-free.
export function useUrlState(): UseUrlState {
  const [state, setState] = useState<UrlState>(() => readState());

  useEffect(() => {
    const onPop = () => setState(readState());
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);

  const setUrlState = useCallback((next: Partial<UrlState>, options?: { replace?: boolean }) => {
    const current = readState();
    const merged: UrlState = {
      q: next.q !== undefined ? next.q.trim() : current.q,
      id: next.id !== undefined ? next.id : current.id,
      lang: next.lang !== undefined ? next.lang.trim() : current.lang,
      best: next.best !== undefined ? next.best : current.best,
      seasons: next.seasons !== undefined ? normalizeSeasons(next.seasons) : current.seasons,
    };

    const params = new URLSearchParams(window.location.search);
    writeParam(params, 'q', merged.q);
    writeParam(params, 'id', merged.id);
    writeParam(params, 'lang', merged.lang);
    writeParam(params, 'best', merged.best);
    writeSeasonsParam(params, merged.seasons);

    const search = params.toString();
    const url = `${window.location.pathname}${search ? `?${search}` : ''}${window.location.hash}`;

    if (url !== `${window.location.pathname}${window.location.search}${window.location.hash}`) {
      if (options?.replace) {
        window.history.replaceState(null, '', url);
      } else {
        window.history.pushState(null, '', url);
      }
    }

    setState((prev) => (statesEqual(prev, merged) ? prev : merged));
  }, []);

  return { ...state, setUrlState };
}
