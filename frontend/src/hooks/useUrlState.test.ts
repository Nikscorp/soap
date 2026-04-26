import { describe, expect, it, beforeEach, afterEach } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { useUrlState } from './useUrlState';

function setLocation(search: string) {
  window.history.replaceState(null, '', `/${search}`);
}

describe('useUrlState', () => {
  beforeEach(() => {
    setLocation('');
  });

  afterEach(() => {
    setLocation('');
  });

  it('returns empty defaults when no params are present', () => {
    const { result } = renderHook(() => useUrlState());
    expect(result.current.q).toBe('');
    expect(result.current.id).toBeNull();
    expect(result.current.lang).toBe('');
    expect(result.current.best).toBeNull();
  });

  it('reads all params from the URL on mount', () => {
    setLocation('?q=lost&id=42&lang=en&best=5');
    const { result } = renderHook(() => useUrlState());
    expect(result.current.q).toBe('lost');
    expect(result.current.id).toBe(42);
    expect(result.current.lang).toBe('en');
    expect(result.current.best).toBe(5);
  });

  it('rejects non-positive numeric params', () => {
    setLocation('?id=-1&best=abc');
    const { result } = renderHook(() => useUrlState());
    expect(result.current.id).toBeNull();
    expect(result.current.best).toBeNull();
  });

  it('setUrlState pushes a new history entry by default', () => {
    const { result } = renderHook(() => useUrlState());
    const before = window.history.length;

    act(() => result.current.setUrlState({ q: 'breaking bad' }));

    expect(result.current.q).toBe('breaking bad');
    expect(window.location.search).toBe('?q=breaking+bad');
    expect(window.history.length).toBe(before + 1);
  });

  it('setUrlState with replace does not grow history', () => {
    const { result } = renderHook(() => useUrlState());
    const before = window.history.length;

    act(() => result.current.setUrlState({ q: 'foo' }, { replace: true }));

    expect(result.current.q).toBe('foo');
    expect(window.history.length).toBe(before);
  });

  it('partial updates preserve unrelated params', () => {
    setLocation('?q=lost&id=42&lang=en&best=5');
    const { result } = renderHook(() => useUrlState());

    act(() => result.current.setUrlState({ best: 7 }));

    expect(result.current.q).toBe('lost');
    expect(result.current.id).toBe(42);
    expect(result.current.lang).toBe('en');
    expect(result.current.best).toBe(7);
  });

  it('setting null/empty strips the corresponding param', () => {
    setLocation('?q=lost&id=42&lang=en&best=5');
    const { result } = renderHook(() => useUrlState());

    act(() => result.current.setUrlState({ q: '', id: null, best: null }));

    expect(window.location.search).toBe('?lang=en');
    expect(result.current.q).toBe('');
    expect(result.current.id).toBeNull();
    expect(result.current.best).toBeNull();
  });

  it('preserves non-managed params already in the URL', () => {
    setLocation('?q=lost&other=keep');
    const { result } = renderHook(() => useUrlState());

    act(() => result.current.setUrlState({ q: 'foo' }));

    expect(window.location.search).toContain('other=keep');
    expect(window.location.search).toContain('q=foo');
  });

  it('responds to popstate (browser back/forward)', () => {
    setLocation('?q=initial');
    const { result } = renderHook(() => useUrlState());
    expect(result.current.q).toBe('initial');

    setLocation('?id=42&lang=en');
    act(() => {
      window.dispatchEvent(new PopStateEvent('popstate'));
    });
    expect(result.current.q).toBe('');
    expect(result.current.id).toBe(42);
    expect(result.current.lang).toBe('en');
  });

  it('no-op when nothing changes does not push history', () => {
    setLocation('?q=lost');
    const { result } = renderHook(() => useUrlState());
    const before = window.history.length;

    act(() => result.current.setUrlState({ q: 'lost' }));

    expect(window.history.length).toBe(before);
  });
});
