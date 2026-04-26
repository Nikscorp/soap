import type { ReactNode } from 'react';

export function EmptyState({ children }: { children: ReactNode }) {
  return (
    <div className="px-6 py-8 text-center text-sm text-slate-600" role="status">
      {children}
    </div>
  );
}
