import { describe, expect, it, beforeEach, afterEach } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { useUrlQuery } from './useUrlQuery';

function setLocation(search: string) {
  window.history.replaceState(null, '', `/${search}`);
}

describe('useUrlQuery', () => {
  beforeEach(() => {
    setLocation('');
  });

  afterEach(() => {
    setLocation('');
  });

  it('returns "" when ?q is absent', () => {
    const { result } = renderHook(() => useUrlQuery());
    expect(result.current.q).toBe('');
  });

  it('reads ?q from the current URL on mount', () => {
    setLocation('?q=lost');
    const { result } = renderHook(() => useUrlQuery());
    expect(result.current.q).toBe('lost');
  });

  it('setQ pushes a new history entry and updates state', () => {
    const { result } = renderHook(() => useUrlQuery());
    const before = window.history.length;

    act(() => result.current.setQ('breaking bad'));

    expect(result.current.q).toBe('breaking bad');
    expect(window.location.search).toBe('?q=breaking+bad');
    expect(window.history.length).toBe(before + 1);
  });

  it('setQ with replace does not grow history', () => {
    const { result } = renderHook(() => useUrlQuery());
    const before = window.history.length;

    act(() => result.current.setQ('foo', { replace: true }));

    expect(result.current.q).toBe('foo');
    expect(window.history.length).toBe(before);
  });

  it('setQ with empty string strips ?q from the URL', () => {
    setLocation('?q=lost&other=keep');
    const { result } = renderHook(() => useUrlQuery());

    act(() => result.current.setQ(''));

    expect(result.current.q).toBe('');
    expect(window.location.search).toBe('?other=keep');
  });

  it('responds to popstate (browser back/forward)', () => {
    setLocation('?q=initial');
    const { result } = renderHook(() => useUrlQuery());
    expect(result.current.q).toBe('initial');

    // Simulate a back-navigation: rewrite the URL, then fire popstate.
    setLocation('?q=earlier');
    act(() => {
      window.dispatchEvent(new PopStateEvent('popstate'));
    });
    expect(result.current.q).toBe('earlier');
  });
});
