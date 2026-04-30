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
    <li className="flex items-start gap-3 px-4 py-3 sm:gap-4 sm:px-8">
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
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline gap-2">
          <span className="flex-none text-xs font-medium tracking-wide text-slate-400 tabular-nums">
            {formatEpisodeCode(episode.season, episode.number)}
          </span>
          <p className="min-w-0 flex-1 text-sm font-semibold break-words text-slate-800">
            {episode.title}
          </p>
          <span
            className="flex flex-none items-center gap-1 text-xs font-semibold text-slate-500 tabular-nums"
            aria-label={`Rating ${formatRating(episode.rating)}`}
          >
            <Star className="h-3.5 w-3.5 fill-yellow-400 text-yellow-400" aria-hidden="true" />
            {formatRating(episode.rating)}
          </span>
        </div>
        {description && (
          <p className="mt-1 text-xs leading-snug break-words text-slate-500">{description}</p>
        )}
      </div>
    </li>
  );
}
