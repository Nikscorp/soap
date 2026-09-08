import { Monitor, Moon, Sun } from 'lucide-react';
import { useTheme, type Theme } from '@/hooks/useTheme';
import { clsx } from '@/lib/clsx';

const options = [
  { value: 'system', label: 'System', Icon: Monitor },
  { value: 'light', label: 'Light', Icon: Sun },
  { value: 'dark', label: 'Dark', Icon: Moon },
] satisfies { value: Theme; label: string; Icon: typeof Monitor }[];

export function ThemeSelector() {
  const { theme, changeTheme } = useTheme();

  return (
    <div
      role="group"
      aria-label="Color theme"
      className="mt-2 inline-flex gap-1 rounded-full border border-border bg-card p-1"
    >
      {options.map(({ value, label, Icon }) => (
        <button
          key={value}
          type="button"
          aria-pressed={theme === value}
          onClick={() => changeTheme(value)}
          className={clsx(
            'inline-flex items-center gap-1.5 rounded-full px-3 py-2 text-xs font-medium transition-colors focus-visible:outline-accent',
            theme === value
              ? 'bg-foreground text-card'
              : 'text-muted hover:bg-surface-hover hover:text-foreground',
          )}
        >
          <Icon className="h-3.5 w-3.5" aria-hidden="true" />
          {label}
        </button>
      ))}
    </div>
  );
}
