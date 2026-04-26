import { useState } from 'react';
import { Star } from 'lucide-react';
import type { Episode } from '@/lib/types';
import { normalizePosterUrl } from '@/lib/api';
import { formatEpisodeCode, formatRating } from '@/lib/format';

interface Props {
  episode: Episode;
}

export function EpisodeRow({ episode }: Props) {
  const description = episode.description?.trim();
  const stillUrl = normalizePosterUrl(episode.still);
  const [stillFailed, setStillFailed] = useState(false);
  const showStill = stillUrl && !stillFailed;
  return (
    <li className="flex items-start justify-between gap-3 px-5 py-3 sm:px-10">
      <div className="flex min-w-0 flex-1 items-start gap-3">
        {showStill && (
          <img
            src={stillUrl}
            alt=""
            loading="lazy"
            decoding="async"
            onError={() => setStillFailed(true)}
            className="h-[60px] w-[100px] flex-none rounded object-cover"
          />
        )}
        <span className="w-12 flex-none pt-0.5 text-xs font-medium tracking-wide text-slate-400 tabular-nums">
          {formatEpisodeCode(episode.season, episode.number)}
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold text-slate-800">{episode.title}</p>
          {description && (
            <p className="mt-0.5 text-xs leading-snug text-slate-500">{description}</p>
          )}
        </div>
      </div>
      <span
        className="flex flex-none items-center gap-1 pt-0.5 text-xs font-semibold text-slate-500 tabular-nums"
        aria-label={`Rating ${formatRating(episode.rating)}`}
      >
        <Star className="h-3.5 w-3.5 fill-yellow-400 text-yellow-400" aria-hidden="true" />
        {formatRating(episode.rating)}
      </span>
    </li>
  );
}
