import { useState } from 'react';
import { ImageOff } from 'lucide-react';
import { normalizePosterUrl, posterSrcSet, type Size } from '@/lib/api';
import { yearFromAirDate } from '@/lib/format';
import { CopyLinkButton } from './CopyLinkButton';

interface Props {
  title: string;
  poster: string;
  firstAirDate: string;
  description?: string;
}

const selectedSeriesPosterSizes: readonly Size[] = ['w185', 'w342'];
const selectedSeriesPosterSizesAttr = '(min-width: 640px) 96px, 80px';

export function SelectedSeriesCard({ title, poster, firstAirDate, description }: Props) {
  const [posterFailed, setPosterFailed] = useState(false);
  const posterUrl = normalizePosterUrl(poster, 'w185');
  const srcSet = posterSrcSet(poster, selectedSeriesPosterSizes) || undefined;
  const year = yearFromAirDate(firstAirDate);

  return (
    <div className="px-5 py-4 sm:px-6">
      <div className="flex items-start gap-4">
        <div className="flex aspect-[2/3] w-20 flex-none items-center justify-center overflow-hidden rounded bg-slate-100 text-slate-400 sm:w-24">
          {posterUrl && !posterFailed ? (
            <img
              src={posterUrl}
              srcSet={srcSet}
              sizes={selectedSeriesPosterSizesAttr}
              alt=""
              fetchPriority="high"
              decoding="async"
              className="h-full w-full object-cover"
              onError={() => setPosterFailed(true)}
            />
          ) : (
            <ImageOff className="h-6 w-6" aria-hidden="true" />
          )}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-start gap-3">
            <div className="min-w-0 flex-1">
              <h2 className="text-base font-semibold break-words text-slate-900 sm:text-lg">
                {title || ' '}
              </h2>
              {year && <p className="text-sm text-slate-500">{year}</p>}
            </div>
            <span className="flex-none">
              <CopyLinkButton />
            </span>
          </div>
          {description && (
            <p className="mt-2 text-sm leading-snug break-words text-slate-600 sm:mt-3">
              {description}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
