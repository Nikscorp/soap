import { useEffect, useRef, useState } from 'react';
import { Check, Link2 } from 'lucide-react';
import { clsx } from '@/lib/clsx';

type Status = 'idle' | 'copied' | 'error';

const FEEDBACK_MS = 1500;

// Copies the current page URL to the clipboard. Uses the modern async
// Clipboard API when available; falls back to a hidden input + execCommand
// for older browsers and non-secure contexts (HTTP, embedded webviews).
async function copyToClipboard(text: string): Promise<void> {
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const el = document.createElement('input');
  el.value = text;
  el.setAttribute('readonly', '');
  el.style.position = 'absolute';
  el.style.left = '-9999px';
  document.body.appendChild(el);
  try {
    el.select();
    const ok = document.execCommand('copy');
    if (!ok) throw new Error('execCommand copy failed');
  } finally {
    document.body.removeChild(el);
  }
}

export function CopyLinkButton() {
  const [status, setStatus] = useState<Status>('idle');
  const timerRef = useRef<number | undefined>(undefined);

  useEffect(() => {
    return () => {
      if (timerRef.current !== undefined) {
        window.clearTimeout(timerRef.current);
      }
    };
  }, []);

  const handleClick = async () => {
    try {
      await copyToClipboard(window.location.href);
      setStatus('copied');
    } catch {
      setStatus('error');
    }
    if (timerRef.current !== undefined) {
      window.clearTimeout(timerRef.current);
    }
    timerRef.current = window.setTimeout(() => setStatus('idle'), FEEDBACK_MS);
  };

  const label =
    status === 'copied' ? 'Copied!' : status === 'error' ? 'Unable to copy' : 'Copy link';
  const Icon = status === 'copied' ? Check : Link2;

  return (
    <button
      type="button"
      onClick={handleClick}
      aria-label="Copy link to this page"
      className={clsx(
        'inline-flex flex-none items-center gap-1.5 rounded-md border border-border px-2.5 py-1.5 text-xs font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-accent',
        status === 'copied'
          ? 'border-success/30 bg-success-soft text-success'
          : status === 'error'
            ? 'border-danger/30 bg-danger-soft text-danger'
            : 'bg-card text-secondary hover:bg-surface-hover hover:text-foreground',
      )}
    >
      <Icon className="h-3.5 w-3.5" aria-hidden="true" />
      <span aria-live="polite">{label}</span>
    </button>
  );
}
