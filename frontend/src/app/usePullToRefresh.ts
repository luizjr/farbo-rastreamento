import { useEffect, useRef, useState } from 'react';
import type { RefObject } from 'react';

import { haptic } from './haptics';

/** Distância (já com a resistência aplicada) que dispara a atualização. */
export const PULL_THRESHOLD = 64;
const MAX_PULL = 120;

/** Resistência do elástico: quanto mais puxa, menos anda. */
export function rubberBand(distance: number): number {
  if (distance <= 0) return 0;
  return MAX_PULL * (1 - Math.exp(-distance / (MAX_PULL * 1.1)));
}

export interface PullState {
  /** Quanto o conteúdo desceu, em px. */
  offset: number;
  refreshing: boolean;
  /** O dedo está puxando (sem animação de volta). */
  pulling: boolean;
}

/**
 * "Puxar para atualizar" no contêiner que rola. O app instalado não tem o
 * botão de recarregar do navegador, então é por aqui que o cliente pede
 * dados novos.
 *
 * Só começa com o contêiner no topo e o gesto na vertical; ignora gestos que
 * nascem no mapa ou dentro de uma janela (modal).
 */
export function usePullToRefresh(
  ref: RefObject<HTMLElement | null>,
  onRefresh: () => Promise<unknown>,
  enabled: boolean,
  resetKey: unknown,
): PullState {
  const [state, setState] = useState<PullState>({ offset: 0, refreshing: false, pulling: false });
  const refreshing = useRef(false);
  const callback = useRef(onRefresh);
  callback.current = onRefresh;

  useEffect(() => {
    const el = ref.current;
    if (!el || !enabled) return;

    let startX = 0;
    let startY = 0;
    let tracking = false;
    let active = false;
    let offset = 0;
    let armed = false;

    const onStart = (event: TouchEvent) => {
      if (refreshing.current || event.touches.length !== 1 || el.scrollTop > 0) return;
      const target = event.target as Element | null;
      if (target?.closest('.leaflet-container, [role="dialog"], [data-no-pull]')) return;
      startX = event.touches[0].clientX;
      startY = event.touches[0].clientY;
      tracking = true;
      active = false;
      armed = false;
    };

    const onMove = (event: TouchEvent) => {
      if (!tracking) return;
      const dx = event.touches[0].clientX - startX;
      const dy = event.touches[0].clientY - startY;
      if (!active) {
        // Gesto lateral (carrossel, tabela) ou para cima: não é conosco.
        if (Math.abs(dx) > Math.abs(dy) || dy <= 0 || el.scrollTop > 0) {
          if (Math.abs(dy) > 8 || Math.abs(dx) > 8) tracking = false;
          return;
        }
        if (dy < 6) return;
        active = true;
      }
      event.preventDefault(); // segura o "quique" nativo enquanto puxa
      offset = rubberBand(dy);
      if (!armed && offset >= PULL_THRESHOLD) {
        armed = true;
        haptic(10);
      } else if (armed && offset < PULL_THRESHOLD) {
        armed = false;
      }
      setState({ offset, refreshing: false, pulling: true });
    };

    const onEnd = async () => {
      if (!tracking) return;
      tracking = false;
      if (!active) return;
      active = false;
      if (offset < PULL_THRESHOLD) {
        setState({ offset: 0, refreshing: false, pulling: false });
        return;
      }
      refreshing.current = true;
      setState({ offset: PULL_THRESHOLD * 0.75, refreshing: true, pulling: false });
      try {
        await callback.current();
      } finally {
        refreshing.current = false;
        setState({ offset: 0, refreshing: false, pulling: false });
      }
    };

    el.addEventListener('touchstart', onStart, { passive: true });
    el.addEventListener('touchmove', onMove, { passive: false });
    el.addEventListener('touchend', onEnd);
    el.addEventListener('touchcancel', onEnd);
    return () => {
      el.removeEventListener('touchstart', onStart);
      el.removeEventListener('touchmove', onMove);
      el.removeEventListener('touchend', onEnd);
      el.removeEventListener('touchcancel', onEnd);
    };
  }, [ref, enabled, resetKey]);

  return state;
}
