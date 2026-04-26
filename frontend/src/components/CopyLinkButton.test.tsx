import { describe, expect, it, beforeEach, afterEach, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { CopyLinkButton } from './CopyLinkButton';

function defineNavigatorClipboard(value: unknown) {
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    writable: true,
    value,
  });
}

describe('CopyLinkButton', () => {
  beforeEach(() => {
    window.history.replaceState(null, '', '/?id=42&lang=en&best=5');
  });

  afterEach(() => {
    window.history.replaceState(null, '', '/');
    defineNavigatorClipboard(undefined);
  });

  it('writes the current URL to the clipboard and shows feedback', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    defineNavigatorClipboard({ writeText });

    render(<CopyLinkButton />);
    fireEvent.click(screen.getByRole('button', { name: /copy link/i }));

    await waitFor(() => expect(writeText).toHaveBeenCalledWith(window.location.href));
    await waitFor(() => expect(screen.getByText('Copied!')).toBeInTheDocument());
  });

  it('falls back to execCommand when navigator.clipboard is unavailable', async () => {
    defineNavigatorClipboard(undefined);
    const execCommand = vi.fn().mockReturnValue(true);
    Object.defineProperty(document, 'execCommand', {
      configurable: true,
      writable: true,
      value: execCommand,
    });

    render(<CopyLinkButton />);
    fireEvent.click(screen.getByRole('button', { name: /copy link/i }));

    await waitFor(() => expect(execCommand).toHaveBeenCalledWith('copy'));
    await waitFor(() => expect(screen.getByText('Copied!')).toBeInTheDocument());
  });

  it('shows an error when the copy fails', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('denied'));
    defineNavigatorClipboard({ writeText });

    render(<CopyLinkButton />);
    fireEvent.click(screen.getByRole('button', { name: /copy link/i }));

    await waitFor(() => expect(screen.getByText('Unable to copy')).toBeInTheDocument());
  });
});
