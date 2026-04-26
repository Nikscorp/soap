import { useQuery } from '@tanstack/react-query';
import { getEpisodesById } from '@/lib/api';
import type { EpisodesResponse, SearchResult } from '@/lib/types';
import { Spinner } from './Spinner';
import { EmptyState } from './EmptyState';
import { ErrorState } from './ErrorState';
import { EpisodeRow } from './EpisodeRow';
import { SelectedSeriesCard } from './SelectedSeriesCard';

interface Props {
  series: SearchResult;
  language: string;
}

export function EpisodesList({ series, language }: Props) {
  const query = useQuery<EpisodesResponse>({
    queryKey: ['episodes', series.id, language],
    queryFn: ({ signal }) => getEpisodesById(series.id, language, signal),
  });

  return (
    <section
      className="mx-auto mt-5 mb-10 w-[95%] max-w-3xl overflow-hidden rounded-md bg-white shadow-card sm:w-[80%]"
      aria-busy={query.isPending}
    >
      <SelectedSeriesCard series={series} />
      <div className="border-t border-slate-100">
        {query.isPending && <Spinner label="Loading best episodes…" />}
        {query.isError && <ErrorState>Service unavailable</ErrorState>}
        {query.isSuccess && <EpisodesBody data={query.data} />}
      </div>
    </section>
  );
}

function EpisodesBody({ data }: { data: EpisodesResponse }) {
  const episodes = data.episodes ?? [];
  if (episodes.length === 0) {
    return (
      <EmptyState>
        We cannot find best episodes because there are no ratings on IMDb.
      </EmptyState>
    );
  }
  return (
    <>
      <h3 className="px-5 pt-5 pb-2 text-xs font-semibold tracking-wider text-slate-500 uppercase sm:px-10">
        Best of &ldquo;{data.title}&rdquo;
      </h3>
      <ul className="divide-y divide-slate-100 pb-3">
        {episodes.map((ep, idx) => (
          <EpisodeRow key={`${ep.season}-${ep.number}-${idx}`} episode={ep} />
        ))}
      </ul>
    </>
  );
}
