import { useEffect, useMemo, useRef, useState } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { getEpisodesById } from '@/lib/api';
import type { Episode, EpisodesResponse, SearchResult } from '@/lib/types';
import { Spinner } from './Spinner';
import { EmptyState } from './EmptyState';
import { ErrorState } from './ErrorState';
import { EpisodeRow } from './EpisodeRow';
import { SelectedSeriesCard } from './SelectedSeriesCard';
import { EpisodesSlider } from './EpisodesSlider';

interface Props {
  series: SearchResult;
  language: string;
}

const REFETCH_DEBOUNCE_MS = 200;

// EpisodesList renders the best-episodes panel for a selected series and a
// slider that lets the user pick how many top-rated episodes to show.
//
// The hosting component should mount this with a stable key per (series,
// language) so the per-series state resets cleanly without a reset effect.
//
// State model (kept intentionally small):
//   * `sliderValue` (UI) — the slider's current position. `null` means
//     "use the server-provided defaultBest" until the user nudges it.
//   * `fetchLimit` (network) — the largest `limit` we've ever fetched. It
//     only ever grows: dragging downwards re-slices locally; dragging past
//     what we have refetches with a bigger limit.
export function EpisodesList({ series, language }: Props) {
  const [sliderValue, setSliderValue] = useState<number | null>(null);
  const [fetchLimit, setFetchLimit] = useState<number | undefined>(undefined);

  const query = useQuery<EpisodesResponse>({
    queryKey: ['episodes', series.id, language, fetchLimit],
    queryFn: ({ signal }) => getEpisodesById(series.id, language, fetchLimit, signal),
    placeholderData: keepPreviousData,
  });

  // Refs are read inside the slider's debounced callback so we don't have to
  // depend on React state there (which would otherwise force an effect that
  // calls setState).
  const refetchTimerRef = useRef<number | undefined>(undefined);
  const fetchedLenRef = useRef(0);
  const fetchedLen = query.data?.episodes?.length ?? 0;
  useEffect(() => {
    fetchedLenRef.current = fetchedLen;
  }, [fetchedLen]);

  useEffect(() => {
    return () => {
      if (refetchTimerRef.current !== undefined) {
        window.clearTimeout(refetchTimerRef.current);
      }
    };
  }, []);

  const handleSliderChange = (next: number) => {
    setSliderValue(next);
    if (refetchTimerRef.current !== undefined) {
      window.clearTimeout(refetchTimerRef.current);
    }
    refetchTimerRef.current = window.setTimeout(() => {
      if (next > fetchedLenRef.current) {
        setFetchLimit((prev) => (prev === next ? prev : next));
      }
    }, REFETCH_DEBOUNCE_MS);
  };

  return (
    <section
      className="mx-auto mt-5 mb-10 w-[95%] max-w-3xl overflow-hidden rounded-md bg-white shadow-card sm:w-[80%]"
      aria-busy={query.isPending}
    >
      <SelectedSeriesCard series={series} />
      <div className="border-t border-slate-100">
        {query.isPending && <Spinner label="Loading best episodes…" />}
        {query.isError && <ErrorState>Service unavailable</ErrorState>}
        {query.isSuccess && query.data && (
          <EpisodesBody
            data={query.data}
            sliderValue={sliderValue}
            onSliderChange={handleSliderChange}
          />
        )}
      </div>
    </section>
  );
}

interface BodyProps {
  data: EpisodesResponse;
  sliderValue: number | null;
  onSliderChange: (next: number) => void;
}

function EpisodesBody({ data, sliderValue, onSliderChange }: BodyProps) {
  const total = data.totalEpisodes;
  const count = sliderValue ?? data.defaultBest;

  // Pick the top-`count` episodes by rating from whatever the server most
  // recently returned, then re-sort chronologically for display. Using
  // server-returned data alone is safe because top-N by rating is monotone
  // in N: a larger response always contains every episode of a smaller one.
  const visible = useMemo(() => {
    const pool = data.episodes ?? [];
    if (pool.length === 0) return [] as Episode[];
    const cap = Math.max(0, Math.min(count, pool.length));
    const byRating = pool.slice().sort((a, b) => b.rating - a.rating);
    const top = byRating.slice(0, cap);
    return top.sort((a, b) => a.season - b.season || a.number - b.number);
  }, [data.episodes, count]);

  if (total === 0) {
    return (
      <EmptyState>
        We cannot find best episodes because there are no ratings on IMDb.
      </EmptyState>
    );
  }

  return (
    <>
      <h3 className="px-5 pt-5 pb-1 text-xs font-semibold tracking-wider text-slate-500 uppercase sm:px-10">
        Best of &ldquo;{data.title}&rdquo;
      </h3>
      {total > 1 && (
        <EpisodesSlider
          value={count}
          min={1}
          max={total}
          total={total}
          onChange={onSliderChange}
        />
      )}
      <ul className="divide-y divide-slate-100 pb-3">
        {visible.map((ep, idx) => (
          <EpisodeRow key={`${ep.season}-${ep.number}-${idx}`} episode={ep} />
        ))}
      </ul>
    </>
  );
}
