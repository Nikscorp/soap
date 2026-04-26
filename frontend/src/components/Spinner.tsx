import { Loader2 } from 'lucide-react';

export function Spinner({ label = 'Loading…' }: { label?: string }) {
  return (
    <div
      className="flex w-full justify-center py-6"
      role="status"
      aria-live="polite"
      aria-label={label}
    >
      <Loader2 className="h-7 w-7 animate-spin text-accent" aria-hidden="true" />
      <span className="sr-only">{label}</span>
    </div>
  );
}
