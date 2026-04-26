import type { MouseEvent } from 'react';

interface HeaderProps {
  onHomeClick: () => void;
}

export function Header({ onHomeClick }: HeaderProps) {
  const handleClick = (e: MouseEvent<HTMLAnchorElement>) => {
    if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    onHomeClick();
  };

  return (
    <header className="flex flex-col items-center justify-center gap-3 px-4 pt-10 pb-6 sm:pt-14">
      <a
        href="/"
        onClick={handleClick}
        className="rounded cursor-pointer transition-opacity hover:opacity-90 focus:outline-none focus-visible:ring-2 focus-visible:ring-white/70"
      >
        <h1 className="text-title text-center text-5xl font-extrabold tracking-[0.2em] text-white sm:text-6xl">
          LAZY SOAP
        </h1>
      </a>
      <p className="text-center text-sm font-light tracking-[0.24em] text-white/70 sm:text-base">
        watch&nbsp;only&nbsp;best episodes&nbsp;of&nbsp;series
      </p>
    </header>
  );
}
