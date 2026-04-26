import { describe, expect, it } from 'vitest';
import { __testing } from './useKeyboardListNav';

const { reducer } = __testing;

describe('useKeyboardListNav reducer', () => {
  const initial = { activeIndex: -1, itemCount: 3 };

  it('starts with no active item', () => {
    expect(initial.activeIndex).toBe(-1);
  });

  it('next moves forward but stops at last index', () => {
    let s = initial;
    s = reducer(s, { type: 'next' });
    expect(s.activeIndex).toBe(0);
    s = reducer(s, { type: 'next' });
    s = reducer(s, { type: 'next' });
    expect(s.activeIndex).toBe(2);
    s = reducer(s, { type: 'next' });
    expect(s.activeIndex).toBe(2);
  });

  it('prev moves backward but stops at -1', () => {
    let s = { ...initial, activeIndex: 1 };
    s = reducer(s, { type: 'prev' });
    expect(s.activeIndex).toBe(0);
    s = reducer(s, { type: 'prev' });
    expect(s.activeIndex).toBe(-1);
    s = reducer(s, { type: 'prev' });
    expect(s.activeIndex).toBe(-1);
  });

  it('set clamps to valid range', () => {
    expect(reducer(initial, { type: 'set', index: 99 }).activeIndex).toBe(2);
    expect(reducer(initial, { type: 'set', index: -5 }).activeIndex).toBe(-1);
    expect(reducer(initial, { type: 'set', index: 1 }).activeIndex).toBe(1);
  });

  it('reset zeroes out activeIndex and updates itemCount', () => {
    const s = { activeIndex: 2, itemCount: 3 };
    expect(reducer(s, { type: 'reset', itemCount: 0 })).toEqual({ activeIndex: -1, itemCount: 0 });
  });

  it('clear sets activeIndex to -1 without changing itemCount', () => {
    const s = { activeIndex: 2, itemCount: 3 };
    expect(reducer(s, { type: 'clear' })).toEqual({ activeIndex: -1, itemCount: 3 });
  });

  it('next/prev no-op when itemCount is 0', () => {
    const s = { activeIndex: -1, itemCount: 0 };
    expect(reducer(s, { type: 'next' })).toEqual(s);
    expect(reducer(s, { type: 'prev' })).toEqual(s);
  });
});
