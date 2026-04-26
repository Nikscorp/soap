import { useId, type ChangeEvent } from 'react';

interface Props {
  value: number;
  min: number;
  max: number;
  total: number;
  onChange: (next: number) => void;
}

export function EpisodesSlider({ value, min, max, total, onChange }: Props) {
  const id = useId();
  const safeMax = Math.max(min, max);
  const safeValue = Math.min(Math.max(value, min), safeMax);
  const disabled = safeMax <= min;

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    const next = Number(e.target.value);
    if (Number.isFinite(next)) onChange(next);
  };

  return (
    <div className="flex flex-col gap-2 px-5 pt-4 pb-2 sm:px-10">
      <label
        htmlFor={id}
        className="flex items-baseline justify-between text-xs font-semibold tracking-wider text-slate-500 uppercase"
      >
        <span>Show top episodes</span>
        <span className="tabular-nums text-slate-700">
          {safeValue} of {total}
        </span>
      </label>
      <input
        id={id}
        type="range"
        min={min}
        max={safeMax}
        step={1}
        value={safeValue}
        disabled={disabled}
        onChange={handleChange}
        aria-label="Number of best episodes to show"
        aria-valuemin={min}
        aria-valuemax={safeMax}
        aria-valuenow={safeValue}
        className="h-2 w-full cursor-pointer appearance-none rounded-full bg-slate-200 accent-accent disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-accent"
      />
    </div>
  );
}
