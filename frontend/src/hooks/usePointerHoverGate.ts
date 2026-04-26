import { useEffect, useState } from 'react';

/**
 * Mouse-hover should not steal selection while the user is keyboard-navigating
 * the dropdown. The original UI used `react-device-detect` to pre-disable
 * hover on mobile. This hook does the same job, lighter:
 *  - hover starts disabled,
 *  - first real pointer move enables it,
 *  - keyboard interactions disable it again until the next pointer move.
 */
export function usePointerHoverGate(): { hoverEnabled: boolean; disable: () => void } {
  const [hoverEnabled, setHoverEnabled] = useState(false);

  useEffect(() => {
    const onPointerMove = () => setHoverEnabled(true);
    window.addEventListener('pointermove', onPointerMove, { passive: true });
    return () => window.removeEventListener('pointermove', onPointerMove);
  }, []);

  return { hoverEnabled, disable: () => setHoverEnabled(false) };
}
