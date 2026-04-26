import type { SearchResult } from '@/lib/types';
import { yearFromAirDate } from '@/lib/format';
import { clsx } from '@/lib/clsx';

interface Props {
  result: SearchResult;
  index: number;
  active: boolean;
  optionId: string;
  onSelect: (result: SearchResult) => void;
  onHover: (index: number) => void;
}

export function SearchResultRow({ result, index, active, optionId, onSelect, onHover }: Props) {
  return (
    <li
      id={optionId}
      role="option"
      aria-selected={active}
      data-active={active || undefined}
      data-index={index}
      onPointerEnter={() => onHover(index)}
      onMouseDown={(e) => {
        // Prevent the input from blurring before we get a chance to commit.
        e.preventDefault();
        onSelect(result);
      }}
      className={clsx(
        'flex cursor-pointer items-baseline justify-between gap-3 px-5 py-2.5 text-sm',
        active ? 'bg-accent text-white' : 'text-slate-800 hover:bg-accent/70 hover:text-white',
      )}
    >
      <span className="truncate font-medium">{result.title}</span>
      <span className={clsx('shrink-0 text-xs', active ? 'text-white/80' : 'text-slate-500')}>
        ({yearFromAirDate(result.firstAirDate)})
      </span>
    </li>
  );
}
