import { Star } from 'lucide-react';
import type { Episode } from '@/lib/types';
import { formatEpisodeCode, formatRating } from '@/lib/format';

interface Props {
  episode: Episode;
}

export function EpisodeRow({ episode }: Props) {
  return (
    <li className="flex items-baseline justify-between gap-3 px-5 py-3 sm:px-10">
      <div className="flex min-w-0 items-baseline gap-3">
        <span className="w-12 flex-none text-xs font-medium tracking-wide text-slate-400 tabular-nums">
          {formatEpisodeCode(episode.season, episode.number)}
        </span>
        <span className="truncate text-sm font-semibold text-slate-800">{episode.title}</span>
      </div>
      <span
        className="flex flex-none items-center gap-1 text-xs font-semibold text-slate-500 tabular-nums"
        aria-label={`Rating ${formatRating(episode.rating)}`}
      >
        <Star className="h-3.5 w-3.5 fill-yellow-400 text-yellow-400" aria-hidden="true" />
        {formatRating(episode.rating)}
      </span>
    </li>
  );
}
