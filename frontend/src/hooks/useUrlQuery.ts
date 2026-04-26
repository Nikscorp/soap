import { useCallback, useEffect, useState } from 'react';

const PARAM = 'q';

function readQ(): string {
  if (typeof window === 'undefined') return '';
  return new URLSearchParams(window.location.search).get(PARAM) ?? '';
}

export interface UseUrlQuery {
  q: string;
  setQ: (next: string, options?: { replace?: boolean }) => void;
}

// useUrlQuery binds the `?q=` search param to React state. Writes go through
// history.pushState (or replaceState when `replace` is set) so back/forward
// works without a routing library; popstate keeps every consumer in sync.
export function useUrlQuery(): UseUrlQuery {
  const [q, setQState] = useState<string>(() => readQ());

  useEffect(() => {
    const onPop = () => setQState(readQ());
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);

  const setQ = useCallback((next: string, options?: { replace?: boolean }) => {
    const params = new URLSearchParams(window.location.search);
    const trimmed = next.trim();
    if (trimmed) {
      params.set(PARAM, trimmed);
    } else {
      params.delete(PARAM);
    }
    const search = params.toString();
    const url = `${window.location.pathname}${search ? `?${search}` : ''}${window.location.hash}`;
    if (options?.replace) {
      window.history.replaceState(null, '', url);
    } else {
      window.history.pushState(null, '', url);
    }
    setQState(trimmed);
  }, []);

  return { q, setQ };
}
