import { useEffect, useMemo, useRef, useState } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { getEpisodesById } from '@/lib/api';
import type { Episode, EpisodesResponse } from '@/lib/types';
import { Spinner } from './Spinner';
import { EmptyState } from './EmptyState';
import { ErrorState } from './ErrorState';
import { EpisodeRow } from './EpisodeRow';
import { SelectedSeriesCard } from './SelectedSeriesCard';
import { EpisodesSlider } from './EpisodesSlider';

export interface EpisodesListHint {
  title: string;
  poster: string;
  firstAirDate: string;
}

interface Props {
  seriesId: number;
  language: string;
  // best is the URL-driven slider position (number of top episodes). null
  // means "use the server-provided defaultBest".
  best: number | null;
  hint: EpisodesListHint | null;
  // onBestChange should write `best` to the URL with replaceState semantics
  // (no history bloat during slider drag). Pass null to reset to default.
  onBestChange: (next: number | null) => void;
}

const REFETCH_DEBOUNCE_MS = 200;
const URL_DEBOUNCE_MS = 300;

// EpisodesList renders the best-episodes panel for a selected series and a
// slider that lets the user pick how many top-rated episodes to show.
//
// The hosting component should mount this with a stable key per (seriesId,
// language) so the per-series state resets cleanly without a reset effect.
//
// State model (kept intentionally small):
//   * `sliderValue` (UI) — the slider's current position. `null` means
//     "use the server-provided defaultBest" until the user nudges it.
//   * `fetchLimit` (network) — the largest `limit` we've ever fetched. It
//     only ever grows: dragging downwards re-slices locally; dragging past
//     what we have refetches with a bigger limit.
export function EpisodesList({ seriesId, language, best, hint, onBestChange }: Props) {
  const [sliderValue, setSliderValue] = useState<number | null>(best);
  const [fetchLimit, setFetchLimit] = useState<number | undefined>(
    best && best > 0 ? best : undefined,
  );

  const query = useQuery<EpisodesResponse>({
    queryKey: ['episodes', seriesId, language, fetchLimit],
    queryFn: ({ signal }) => getEpisodesById(seriesId, language, fetchLimit, signal),
    placeholderData: keepPreviousData,
  });

  const refetchTimerRef = useRef<number | undefined>(undefined);
  const urlTimerRef = useRef<number | undefined>(undefined);
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
      if (urlTimerRef.current !== undefined) {
        window.clearTimeout(urlTimerRef.current);
      }
    };
  }, []);

  const defaultBest = query.data?.defaultBest;

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

    if (urlTimerRef.current !== undefined) {
      window.clearTimeout(urlTimerRef.current);
    }
    urlTimerRef.current = window.setTimeout(() => {
      // Omit `best` from the URL when it equals the server default so shared
      // links stay short.
      onBestChange(defaultBest !== undefined && next === defaultBest ? null : next);
    }, URL_DEBOUNCE_MS);
  };

  const cardTitle = query.data?.title ?? hint?.title ?? '';
  const cardPoster = query.data?.poster ?? hint?.poster ?? '';
  const cardYear = query.data?.firstAirDate ?? hint?.firstAirDate ?? '';

  return (
    <section
      className="mx-auto mt-5 mb-10 w-[95%] max-w-3xl overflow-hidden rounded-md bg-white shadow-card sm:w-[80%]"
      aria-busy={query.isPending}
    >
      <SelectedSeriesCard title={cardTitle} poster={cardPoster} firstAirDate={cardYear} />
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
  const rawCount = sliderValue ?? data.defaultBest;
  const count = Math.max(0, Math.min(rawCount, total));

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
      <EmptyState>We cannot find best episodes because there are no ratings on IMDb.</EmptyState>
    );
  }

  return (
    <>
      <h3 className="px-5 pt-5 pb-1 text-xs font-semibold tracking-wider text-slate-500 uppercase sm:px-10">
        Best of &ldquo;{data.title}&rdquo;
      </h3>
      {total > 1 && (
        <EpisodesSlider value={count} min={1} max={total} total={total} onChange={onSliderChange} />
      )}
      <ul className="divide-y divide-slate-100 pb-3">
        {visible.map((ep, idx) => (
          <EpisodeRow key={`${ep.season}-${ep.number}-${idx}`} episode={ep} />
        ))}
      </ul>
    </>
  );
}
