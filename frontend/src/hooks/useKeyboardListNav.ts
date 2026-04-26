import { useCallback, useEffect, useReducer, useRef } from 'react';

type State = { activeIndex: number; itemCount: number };

type Action =
  | { type: 'reset'; itemCount: number }
  | { type: 'set'; index: number }
  | { type: 'next' }
  | { type: 'prev' }
  | { type: 'clear' };

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'reset':
      return { activeIndex: -1, itemCount: action.itemCount };
    case 'set': {
      if (state.itemCount <= 0) return state;
      const clamped = Math.max(-1, Math.min(action.index, state.itemCount - 1));
      return { ...state, activeIndex: clamped };
    }
    case 'next': {
      if (state.itemCount <= 0) return state;
      if (state.activeIndex >= state.itemCount - 1) return state;
      return { ...state, activeIndex: state.activeIndex + 1 };
    }
    case 'prev': {
      if (state.itemCount <= 0) return state;
      if (state.activeIndex <= -1) return state;
      return { ...state, activeIndex: state.activeIndex - 1 };
    }
    case 'clear':
      return { ...state, activeIndex: -1 };
  }
}

export interface UseKeyboardListNav {
  activeIndex: number;
  setActiveIndex: (index: number) => void;
  next: () => void;
  prev: () => void;
  clear: () => void;
}

/**
 * Pure-state combobox keyboard navigation:
 *  - tracks an active index in [0, itemCount-1] or -1 (none).
 *  - resets to -1 whenever itemCount changes.
 */
export function useKeyboardListNav(itemCount: number): UseKeyboardListNav {
  const [state, dispatch] = useReducer(reducer, { activeIndex: -1, itemCount });
  const lastCount = useRef(itemCount);

  useEffect(() => {
    if (lastCount.current !== itemCount) {
      lastCount.current = itemCount;
      dispatch({ type: 'reset', itemCount });
    }
  }, [itemCount]);

  const setActiveIndex = useCallback((index: number) => dispatch({ type: 'set', index }), []);
  const next = useCallback(() => dispatch({ type: 'next' }), []);
  const prev = useCallback(() => dispatch({ type: 'prev' }), []);
  const clear = useCallback(() => dispatch({ type: 'clear' }), []);

  return { activeIndex: state.activeIndex, setActiveIndex, next, prev, clear };
}

// Exported for unit testing.
export const __testing = { reducer };
