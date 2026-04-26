import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { Header } from './Header';

describe('Header', () => {
  it('calls onHomeClick on plain left-click and prevents navigation', () => {
    const onHomeClick = vi.fn();
    render(<Header onHomeClick={onHomeClick} />);

    const link = screen.getByRole('link', { name: /lazy soap/i });
    const event = new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 });
    fireEvent(link, event);

    expect(onHomeClick).toHaveBeenCalledTimes(1);
    expect(event.defaultPrevented).toBe(true);
  });

  it('does not call onHomeClick on cmd/ctrl/shift/alt-click (lets the browser handle it)', () => {
    const onHomeClick = vi.fn();
    render(<Header onHomeClick={onHomeClick} />);
    const link = screen.getByRole('link', { name: /lazy soap/i });

    for (const modifier of ['metaKey', 'ctrlKey', 'shiftKey', 'altKey'] as const) {
      const event = new MouseEvent('click', {
        bubbles: true,
        cancelable: true,
        button: 0,
        [modifier]: true,
      });
      fireEvent(link, event);
      expect(event.defaultPrevented).toBe(false);
    }

    expect(onHomeClick).not.toHaveBeenCalled();
  });

  it('does not call onHomeClick on middle-click', () => {
    const onHomeClick = vi.fn();
    render(<Header onHomeClick={onHomeClick} />);
    const link = screen.getByRole('link', { name: /lazy soap/i });

    const event = new MouseEvent('click', { bubbles: true, cancelable: true, button: 1 });
    fireEvent(link, event);

    expect(onHomeClick).not.toHaveBeenCalled();
    expect(event.defaultPrevented).toBe(false);
  });
});
