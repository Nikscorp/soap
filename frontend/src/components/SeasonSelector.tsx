import { Check } from 'lucide-react';
import { clsx } from '@/lib/clsx';

interface Props {
  available: number[];
  selected: number[] | null;
  onChange: (next: number[] | null) => void;
}

// SeasonSelector renders a chip row for filtering "best episodes" by season.
// `selected === null` is the "all seasons" sentinel (mirrors the URL/API
// contract where an absent `seasons` param means no filter). Clicking a
// season chip from all-mode selects ONLY that season — the common
// "give me episodes from S3 only" path used to require N-1 clicks. From an
// explicit list, clicking a chip toggles it; clicking "All" returns to null.
// When the explicit list happens to cover every available season we
// normalize back to null so the URL stays clean.
export function SeasonSelector({ available, selected, onChange }: Props) {
  if (available.length <= 1) return null;

  // Sanitize `selected` against `available`: a URL like ?seasons=1,99 for a
  // series whose seasons are [1,2,3] would otherwise let toggling either
  // emit an unmappable list (e.g. clicking S1 → onChange([99]) → backend 400)
  // or prematurely collapse to "All" (clicking S2 → [1,99,2] of length 3 ===
  // available.length → onChange(null)). Drop stale entries before any toggle
  // math runs.
  const availableSet = new Set(available);
  const sanitizedSelected =
    selected === null ? null : selected.filter((s) => availableSet.has(s));
  // Normalize to null if the sanitized list covers every available season
  // (handles shared URLs like ?seasons=1,2 when available is exactly [1,2]).
  const effectiveSelected =
    sanitizedSelected !== null && sanitizedSelected.length === available.length
      ? null
      : sanitizedSelected;
  const allSelected = effectiveSelected === null;
  const selectedSet = allSelected ? null : new Set(effectiveSelected);
  // In all-mode only the "All" chip renders pressed. Showing every season
  // chip pressed would contradict the "click chip = select only" behavior.
  const isPressed = (season: number) => selectedSet?.has(season) ?? false;

  const toggleAll = () => {
    if (allSelected) return;
    onChange(null);
  };

  const toggleSeason = (season: number) => {
    let next: number[];
    if (allSelected) {
      next = [season];
    } else {
      const current = effectiveSelected ?? [];
      next = current.includes(season)
        ? current.filter((s) => s !== season)
        : [...current, season].sort((a, b) => a - b);
    }
    if (next.length === 0 || next.length === available.length) {
      onChange(null);
      return;
    }
    onChange(next);
  };

  return (
    <div className="flex flex-col gap-2 px-5 pt-4 pb-1 sm:px-10">
      <span className="text-xs font-semibold tracking-wider text-slate-500 uppercase">
        Filter seasons
      </span>
      <div className="flex flex-wrap gap-2" role="group" aria-label="Filter seasons">
        <Chip pressed={allSelected} onClick={toggleAll} label="All" />
        {available.map((season) => (
          <Chip
            key={season}
            pressed={isPressed(season)}
            onClick={() => toggleSeason(season)}
            label={`S${season}`}
          />
        ))}
      </div>
    </div>
  );
}

interface ChipProps {
  pressed: boolean;
  onClick: () => void;
  label: string;
}

function Chip({ pressed, onClick, label }: ChipProps) {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      onClick={onClick}
      className={clsx(
        'inline-flex items-center gap-1 rounded-full border px-3 py-1 text-xs font-medium tabular-nums transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-accent',
        pressed
          ? 'border-slate-900 bg-slate-900 text-white'
          : 'border-slate-200 bg-white text-slate-700 hover:bg-slate-50 hover:text-slate-900',
      )}
    >
      {pressed && <Check className="h-3.5 w-3.5" aria-hidden="true" />}
      {label}
    </button>
  );
}
