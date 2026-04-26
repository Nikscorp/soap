import { AlertTriangle } from 'lucide-react';
import type { ReactNode } from 'react';

export function ErrorState({ children }: { children: ReactNode }) {
  return (
    <div
      className="flex items-start gap-3 px-6 py-8 text-sm text-red-700"
      role="alert"
      aria-live="assertive"
    >
      <AlertTriangle className="mt-0.5 h-5 w-5 flex-none" aria-hidden="true" />
      <p>{children}</p>
    </div>
  );
}
