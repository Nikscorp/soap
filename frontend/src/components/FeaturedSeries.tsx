import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ImageOff } from 'lucide-react';
import { getFeaturedSeries, normalizePosterUrl, posterSrcSet, type Size } from '@/lib/api';
import { yearFromAirDate } from '@/lib/format';
import type { FeaturedResponse, FeaturedSeries as FeaturedSeriesT, SearchResult } from '@/lib/types';

const featuredPosterSizes: readonly Size[] = ['w342', 'w500', 'w780'];
const featuredPosterSizesAttr =
  '(min-width: 1024px) 210px, (min-width: 640px) 190px, 160px';
// First two carousel cards sit above the fold on a phone — preload them.
const featuredPriorityCount = 2;

interface Props {
  language: string;
  onSelect: (result: SearchResult, language: string) => void;
}

// FeaturedSeries renders a small set of randomized TV series under the search
// bar on the home view. The backend reshuffles per request, so refreshing the
// page surfaces a different selection. Failures and empty responses render
// nothing — this is decorative, not load-bearing.
export function FeaturedSeries({ language, onSelect }: Props) {
  const query = useQuery<FeaturedResponse>({
    queryKey: ['featured', language || 'en'],
    queryFn: ({ signal }) => getFeaturedSeries(language || 'en', signal),
    // Each mount = fresh shuffle. Don't reuse a cached payload across
    // home-page visits within the same session.
    staleTime: 0,
    gcTime: 0,
  });

  const responseLanguage = query.data?.language ?? language ?? 'en';
  const series = query.data?.series ?? [];

  if (query.isError) return null;
  if (query.isSuccess && series.length === 0) return null;

  return (
    <section
      className="mt-5 w-[95%] max-w-3xl sm:w-[80%]"
      aria-labelledby="featured-heading"
    >
      <h2
        id="featured-heading"
        className="mb-2 text-sm font-medium text-white/80"
      >
        Or try one of these
      </h2>
      <ul
        className="-mx-4 flex gap-3 overflow-x-auto px-4 pb-2 sm:mx-0 sm:grid sm:grid-cols-3 sm:gap-4 sm:overflow-visible sm:px-0 sm:pb-0"
      >
        {query.isPending
          ? [0, 1, 2].map((i) => (
              <li key={i} className="w-40 flex-none sm:w-auto">
                <div className="aspect-[2/3] animate-pulse rounded bg-white/10" />
                <div className="mt-2 h-4 w-3/4 animate-pulse rounded bg-white/10" />
                <div className="mt-1 h-3 w-1/3 animate-pulse rounded bg-white/10" />
              </li>
            ))
          : series.map((s, i) => (
              <FeaturedCard
                key={s.id}
                series={s}
                priority={i < featuredPriorityCount}
                onSelect={() =>
                  onSelect(
                    {
                      id: s.id,
                      title: s.title,
                      firstAirDate: s.firstAirDate,
                      poster: s.poster,
                      rating: 0,
                      description: '',
                    },
                    responseLanguage,
                  )
                }
              />
            ))}
      </ul>
    </section>
  );
}

function FeaturedCard({
  series,
  priority,
  onSelect,
}: {
  series: FeaturedSeriesT;
  priority: boolean;
  onSelect: () => void;
}) {
  const [posterFailed, setPosterFailed] = useState(false);
  // Card spans ~160px on mobile and ~190-210px on desktop; the width-
  // descriptor srcset + matching sizes attribute lets the browser pick the
  // smallest rendition that satisfies layout × DPR.
  const posterUrl = normalizePosterUrl(series.poster, 'w342');
  const srcSet = posterSrcSet(series.poster, featuredPosterSizes) || undefined;
  const year = yearFromAirDate(series.firstAirDate);

  return (
    <li className="w-40 flex-none sm:w-auto">
      <button
        type="button"
        onClick={onSelect}
        className="block w-full text-left transition-opacity hover:opacity-90 focus:outline-none focus-visible:ring-2 focus-visible:ring-white/70 rounded"
      >
        <div className="flex aspect-[2/3] w-full items-center justify-center overflow-hidden rounded bg-slate-100 text-slate-400 shadow-card">
          {posterUrl && !posterFailed ? (
            <img
              src={posterUrl}
              srcSet={srcSet}
              sizes={featuredPosterSizesAttr}
              alt=""
              loading={priority ? 'eager' : 'lazy'}
              fetchPriority={priority ? 'high' : 'auto'}
              decoding="async"
              className="h-full w-full object-cover"
              onError={() => setPosterFailed(true)}
            />
          ) : (
            <ImageOff className="h-6 w-6" aria-hidden="true" />
          )}
        </div>
        <h3 className="mt-2 truncate text-sm font-semibold text-white">
          {series.title}
        </h3>
        {year && <p className="text-xs text-white/70">{year}</p>}
      </button>
    </li>
  );
}
