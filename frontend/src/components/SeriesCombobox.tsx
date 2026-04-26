import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Search, X } from 'lucide-react';
import { searchSeries } from '@/lib/api';
import type { SearchResponse, SearchResult } from '@/lib/types';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { useKeyboardListNav } from '@/hooks/useKeyboardListNav';
import { usePointerHoverGate } from '@/hooks/usePointerHoverGate';
import { SearchResultRow } from './SearchResultRow';
import { Spinner } from './Spinner';
import { clsx } from '@/lib/clsx';

const MIN_QUERY_LEN = 2;
const DEBOUNCE_MS = 200;

interface Props {
  onSelect: (result: SearchResult, language: string) => void;
}

export function SeriesCombobox({ onSelect }: Props) {
  const [input, setInput] = useState('');
  const [open, setOpen] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const listboxId = useId();
  const labelId = useId();

  const debounced = useDebouncedValue(input.trim(), DEBOUNCE_MS);
  const enabled = debounced.length >= MIN_QUERY_LEN;

  const query = useQuery<SearchResponse>({
    queryKey: ['search', debounced],
    queryFn: ({ signal }) => searchSeries(debounced, signal),
    enabled,
  });

  const results = useMemo(
    () => (enabled ? (query.data?.searchResults ?? []) : []),
    [enabled, query.data],
  );
  const language = query.data?.language ?? 'en';

  const { activeIndex, setActiveIndex, next, prev, clear } = useKeyboardListNav(results.length);
  const { hoverEnabled, disable: disableHover } = usePointerHoverGate();

  // Open dropdown when there are results and the input has focus.
  useEffect(() => {
    if (results.length > 0 && document.activeElement === inputRef.current) {
      setOpen(true);
    }
  }, [results.length]);

  const commit = (result: SearchResult) => {
    setInput('');
    setOpen(false);
    clear();
    onSelect(result, language);
    inputRef.current?.blur();
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      if (results.length === 0) return;
      e.preventDefault();
      disableHover();
      setOpen(true);
      next();
      return;
    }
    if (e.key === 'ArrowUp') {
      if (results.length === 0) return;
      e.preventDefault();
      disableHover();
      prev();
      return;
    }
    if (e.key === 'Escape') {
      e.preventDefault();
      setOpen(false);
      clear();
      return;
    }
    if (e.key === 'Enter') {
      e.preventDefault();
      const target =
        activeIndex >= 0 && activeIndex < results.length ? results[activeIndex] : results[0];
      if (target) commit(target);
    }
  };

  const onInputChange = (value: string) => {
    setInput(value);
    setOpen(true);
    clear();
  };

  const showDropdown = open && enabled && (query.isFetching || results.length > 0);
  const optionId = activeIndex >= 0 ? `${listboxId}-opt-${activeIndex}` : undefined;

  return (
    <div className="relative w-full">
      <label id={labelId} htmlFor={`${listboxId}-input`} className="sr-only">
        Search TV series
      </label>
      <div className="flex items-center gap-3 border-b border-accent/50 focus-within:border-accent">
        <Search className="h-5 w-5 flex-none text-accent" aria-hidden="true" />
        <input
          ref={inputRef}
          id={`${listboxId}-input`}
          type="text"
          autoComplete="off"
          spellCheck={false}
          enterKeyHint="search"
          placeholder="What series are you looking for?"
          className="w-full bg-transparent py-3 text-lg text-slate-900 placeholder:text-slate-400 focus:outline-none"
          value={input}
          onChange={(e) => onInputChange(e.target.value)}
          onFocus={() => setOpen(true)}
          onBlur={() => {
            window.setTimeout(() => setOpen(false), 100);
          }}
          onKeyDown={onKeyDown}
          role="combobox"
          aria-expanded={showDropdown}
          aria-controls={listboxId}
          aria-autocomplete="list"
          aria-activedescendant={optionId}
          aria-labelledby={labelId}
        />
        {input && (
          <button
            type="button"
            aria-label="Clear search"
            className="rounded p-1 text-slate-400 hover:text-slate-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            onClick={() => {
              setInput('');
              clear();
              setOpen(false);
              inputRef.current?.focus();
            }}
          >
            <X className="h-4 w-4" aria-hidden="true" />
          </button>
        )}
      </div>

      <div
        className={clsx(
          'absolute left-0 right-0 z-10 mt-2 origin-top overflow-hidden rounded-md bg-white shadow-card transition duration-200 ease-out',
          showDropdown
            ? 'visible translate-y-0 opacity-100'
            : 'invisible -translate-y-2 opacity-0',
        )}
      >
        <ul
          id={listboxId}
          role="listbox"
          aria-labelledby={labelId}
          className="max-h-72 overflow-y-auto py-1"
          onMouseLeave={() => clear()}
          data-hover-enabled={hoverEnabled || undefined}
        >
          {query.isFetching && results.length === 0 && (
            <li className="py-2" role="presentation">
              <Spinner label="Searching…" />
            </li>
          )}
          {results.map((result, index) => (
            <SearchResultRow
              key={`${result.id}-${index}`}
              result={result}
              index={index}
              active={index === activeIndex}
              optionId={`${listboxId}-opt-${index}`}
              onSelect={commit}
              onHover={(i) => {
                if (hoverEnabled) setActiveIndex(i);
              }}
            />
          ))}
        </ul>
      </div>

      {query.isError && (
        <p className="mt-2 text-xs text-red-700" role="alert">
          Service unavailable
        </p>
      )}
    </div>
  );
}
