import { useState } from 'react';
import { ImageOff, Star } from 'lucide-react';
import { normalizePosterUrl } from '@/lib/api';
import { yearFromAirDate, formatRating } from '@/lib/format';
import type { SearchResult } from '@/lib/types';

interface Props {
  result: SearchResult;
  onSelect: (result: SearchResult) => void;
}

export function SearchResultCard({ result, onSelect }: Props) {
  const [posterFailed, setPosterFailed] = useState(false);
  const posterUrl = normalizePosterUrl(result.poster);
  const year = yearFromAirDate(result.firstAirDate);
  const description = result.description?.trim();

  return (
    <li>
      <button
        type="button"
        onClick={() => onSelect(result)}
        className="flex w-full items-start gap-4 px-5 py-4 text-left transition-colors hover:bg-accent/5 focus:bg-accent/5 focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent sm:px-6"
      >
        <div className="flex h-[120px] w-[80px] flex-none items-center justify-center overflow-hidden rounded bg-slate-100 text-slate-400 sm:h-[135px] sm:w-[90px]">
          {posterUrl && !posterFailed ? (
            <img
              src={posterUrl}
              alt=""
              loading="lazy"
              decoding="async"
              className="h-full w-full object-cover"
              onError={() => setPosterFailed(true)}
            />
          ) : (
            <ImageOff className="h-6 w-6" aria-hidden="true" />
          )}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline gap-2">
            <h3 className="truncate text-base font-semibold text-slate-900 sm:text-lg">
              {result.title}
            </h3>
            {year && <span className="shrink-0 text-sm text-slate-500">({year})</span>}
          </div>
          <div
            className="mt-1 flex items-center gap-1 text-xs font-semibold text-slate-500 tabular-nums"
            aria-label={`Rating ${formatRating(result.rating)}`}
          >
            <Star className="h-3.5 w-3.5 fill-yellow-400 text-yellow-400" aria-hidden="true" />
            {formatRating(result.rating)}
          </div>
          {description ? (
            <p className="mt-2 line-clamp-3 text-sm leading-snug text-slate-600">{description}</p>
          ) : (
            <p className="mt-2 text-sm text-slate-400 italic">No description available.</p>
          )}
        </div>
      </button>
    </li>
  );
}
