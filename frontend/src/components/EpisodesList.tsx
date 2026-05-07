import { useMemo, useRef, useState, useEffect } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { getEpisodesById, ApiError } from '@/lib/api';
import type { Episode, EpisodesResponse } from '@/lib/types';
import { Spinner } from './Spinner';
import { EmptyState } from './EmptyState';
import { ErrorState } from './ErrorState';
import { EpisodeRow } from './EpisodeRow';
import { SelectedSeriesCard } from './SelectedSeriesCard';
import { EpisodesSlider } from './EpisodesSlider';
import { SeasonSelector } from './SeasonSelector';

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
  // selectedSeasons is the URL-driven season filter; null means "all
  // seasons" (mirrors the API contract where an absent `seasons` param
  // means no filter).
  selectedSeasons: number[] | null;
  hint: EpisodesListHint | null;
  // onBestChange should write `best` to the URL with replaceState semantics
  // (no history bloat during slider drag). Pass null to reset to default.
  onBestChange: (next: number | null) => void;
  // onSeasonsChange writes `seasons` to the URL. SeasonSelector normalizes
  // empty and full-coverage selections to null before calling this.
  onSeasonsChange: (next: number[] | null) => void;
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
export function EpisodesList({
  seriesId,
  language,
  best,
  selectedSeasons,
  hint,
  onBestChange,
  onSeasonsChange,
}: Props) {
  // Stable cache-key string for the React Query key and for detecting
  // selection changes within a single mount.
  const seasonsKey = selectedSeasons?.join(',') ?? 'all';

  const [sliderValue, setSliderValue] = useState<number | null>(best);
  const [fetchLimit, setFetchLimit] = useState<number | undefined>(
    best && best > 0 ? best : undefined,
  );
  // Reset local slider/fetch state on season-set change *during render* so the
  // queryKey on this render already reflects the reset — otherwise a stale
  // fetchLimit briefly leaks into the new request as `?limit=…&seasons=…`.
  // The monotone-in-N invariant the local re-slice exploits ("a larger
  // response always contains every episode of a smaller one") only holds
  // within a fixed season set, so let the new response's defaultBest drive
  // the slider from scratch.
  const refetchTimerRef = useRef<number | undefined>(undefined);
  const urlTimerRef = useRef<number | undefined>(undefined);
  const fetchedLenRef = useRef(0);
  // Survives a 400 error so the chip row stays visible and users can toggle
  // their way back to a valid season combination without using "Show all".
  // State (not a ref) because it's read during render in the error branch.
  const [lastAvailableSeasons, setLastAvailableSeasons] = useState<number[]>([]);

  const [trackedSeasonsKey, setTrackedSeasonsKey] = useState(seasonsKey);
  let effectiveFetchLimit = fetchLimit;
  let effectiveSliderValue = sliderValue;
  if (trackedSeasonsKey !== seasonsKey) {
    setTrackedSeasonsKey(seasonsKey);
    setFetchLimit(undefined);
    setSliderValue(null);
    effectiveFetchLimit = undefined;
    effectiveSliderValue = null;
  }

  const query = useQuery<EpisodesResponse>({
    queryKey: ['episodes', seriesId, language, seasonsKey, effectiveFetchLimit],
    queryFn: ({ signal }) =>
      getEpisodesById(seriesId, language, effectiveFetchLimit, selectedSeasons, signal),
    placeholderData: keepPreviousData,
  });

  // Empty selection is forbidden at this layer: the API contract treats it
  // as "all" and we want the URL to drop the param entirely.
  // Timers are cleared synchronously here so a URL debounce that fires in the
  // same macrotask cycle as a chip click can't write a stale `best` value back
  // to the URL after onSeasonsChange has already reset it.
  const handleSeasonsChange = (next: number[] | null) => {
    if (refetchTimerRef.current !== undefined) {
      window.clearTimeout(refetchTimerRef.current);
      refetchTimerRef.current = undefined;
    }
    if (urlTimerRef.current !== undefined) {
      window.clearTimeout(urlTimerRef.current);
      urlTimerRef.current = undefined;
    }
    onSeasonsChange(next);
  };

  const fetchedLen = query.data?.episodes?.length ?? 0;
  useEffect(() => {
    fetchedLenRef.current = fetchedLen;
  }, [fetchedLen]);

  const availableSeasons = query.data?.availableSeasons;
  if (availableSeasons && availableSeasons !== lastAvailableSeasons) {
    setLastAvailableSeasons(availableSeasons);
  }

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

  // Cancel any pending slider work when the season set changes. Without
  // this, a chip toggle during the 300ms URL debounce lets the deferred
  // onBestChange clobber the `best: null` reset that onSeasonsChange just
  // wrote — and the deferred setFetchLimit leaks the dragged limit into
  // the new season's query.
  // fetchedLenRef is also reset here so a slider drag that fires before the
  // new season's response lands doesn't compare against the old (possibly
  // larger) count and skip the refetch it needs.
  useEffect(() => {
    fetchedLenRef.current = 0;
    if (refetchTimerRef.current !== undefined) {
      window.clearTimeout(refetchTimerRef.current);
      refetchTimerRef.current = undefined;
    }
    if (urlTimerRef.current !== undefined) {
      window.clearTimeout(urlTimerRef.current);
      urlTimerRef.current = undefined;
    }
  }, [seasonsKey]);

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
  const cardDescription = query.data?.description ?? '';

  return (
    <section
      className="mx-auto mt-5 mb-10 w-[95%] max-w-3xl overflow-hidden rounded-md bg-white shadow-card sm:w-[80%]"
      aria-busy={query.isPending}
    >
      <SelectedSeriesCard
        title={cardTitle}
        poster={cardPoster}
        firstAirDate={cardYear}
        description={cardDescription}
      />
      <div className="border-t border-slate-100">
        {query.isPending && <Spinner label="Loading best episodes…" />}
        {query.isError &&
          (query.error instanceof ApiError &&
          query.error.status === 400 &&
          selectedSeasons !== null ? (
            <>
              <SeasonSelector
                available={lastAvailableSeasons}
                selected={selectedSeasons}
                onChange={handleSeasonsChange}
              />
              <ErrorState>
                No episodes found for the selected seasons.{' '}
                <button className="underline" onClick={() => handleSeasonsChange(null)}>
                  Show all seasons
                </button>
              </ErrorState>
            </>
          ) : (
            <ErrorState>Service unavailable</ErrorState>
          ))}
        {query.isSuccess && query.data && (
          <EpisodesBody
            data={query.data}
            sliderValue={effectiveSliderValue}
            onSliderChange={handleSliderChange}
            selectedSeasons={selectedSeasons}
            onSeasonsChange={handleSeasonsChange}
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
  selectedSeasons: number[] | null;
  onSeasonsChange: (next: number[] | null) => void;
}

function EpisodesBody({
  data,
  sliderValue,
  onSliderChange,
  selectedSeasons,
  onSeasonsChange,
}: BodyProps) {
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

  const available = data.availableSeasons ?? [];

  return (
    <>
      <h3 className="px-5 pt-5 pb-1 text-xs font-semibold tracking-wider text-slate-500 uppercase sm:px-10">
        Best of &ldquo;{data.title}&rdquo;
      </h3>
      <SeasonSelector
        available={available}
        selected={selectedSeasons}
        onChange={onSeasonsChange}
      />
      {total === 0 ? (
        <EmptyState>
          We cannot find best episodes because there are no ratings on IMDb.
        </EmptyState>
      ) : (
        <>
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
      )}
    </>
  );
}
