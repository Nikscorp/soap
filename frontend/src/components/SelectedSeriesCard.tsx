import { useState } from 'react';
import { ImageOff } from 'lucide-react';
import { normalizePosterUrl } from '@/lib/api';
import { yearFromAirDate } from '@/lib/format';
import { CopyLinkButton } from './CopyLinkButton';

interface Props {
  title: string;
  poster: string;
  firstAirDate: string;
}

export function SelectedSeriesCard({ title, poster, firstAirDate }: Props) {
  const [posterFailed, setPosterFailed] = useState(false);
  const posterUrl = normalizePosterUrl(poster);
  const year = yearFromAirDate(firstAirDate);

  return (
    <div className="flex items-center gap-4 px-5 py-4 sm:px-6">
      <div className="flex h-[60px] w-[100px] flex-none items-center justify-center overflow-hidden rounded bg-slate-100 text-slate-400">
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
        <h2 className="truncate text-base font-semibold text-slate-900 sm:text-lg">
          {title || ' '}
        </h2>
        {year && <p className="text-sm text-slate-500">{year}</p>}
      </div>
      <CopyLinkButton />
    </div>
  );
}
