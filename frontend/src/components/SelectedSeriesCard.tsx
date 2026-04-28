import { useState } from 'react';
import { ImageOff } from 'lucide-react';
import { normalizePosterUrl } from '@/lib/api';
import { yearFromAirDate } from '@/lib/format';
import { CopyLinkButton } from './CopyLinkButton';

interface Props {
  title: string;
  poster: string;
  firstAirDate: string;
  description?: string;
}

export function SelectedSeriesCard({ title, poster, firstAirDate, description }: Props) {
  const [posterFailed, setPosterFailed] = useState(false);
  // Poster renders at ~80px wide on mobile and ~96px on desktop. w185 covers
  // the 1x case crisply; w342 is the 2x source for HiDPI / Retina screens.
  const posterUrl = normalizePosterUrl(poster, 'w185');
  const posterUrl2x = normalizePosterUrl(poster, 'w342');
  const srcSet = posterUrl && posterUrl2x ? `${posterUrl} 1x, ${posterUrl2x} 2x` : undefined;
  const year = yearFromAirDate(firstAirDate);

  return (
    <div className="flow-root px-5 py-4 sm:px-6">
      <span className="float-right ml-3">
        <CopyLinkButton />
      </span>
      <div className="float-left mr-4 flex aspect-[2/3] w-20 items-center justify-center overflow-hidden rounded bg-slate-100 text-slate-400 sm:w-24">
        {posterUrl && !posterFailed ? (
          <img
            src={posterUrl}
            srcSet={srcSet}
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
      <h2 className="text-base font-semibold break-words text-slate-900 sm:text-lg">
        {title || ' '}
      </h2>
      {year && <p className="text-sm text-slate-500">{year}</p>}
      {description && (
        <p className="mt-2 text-sm leading-snug break-words text-slate-600">
          {description}
        </p>
      )}
    </div>
  );
}
