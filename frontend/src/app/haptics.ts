/**
 * Vibração curta de confirmação (Android). O Safari do iPhone não expõe
 * vibração para a web; lá a chamada simplesmente não faz nada.
 */
export function haptic(ms = 8): void {
  try {
    if (window.matchMedia?.('(pointer: coarse)').matches) navigator.vibrate?.(ms);
  } catch {
    /* sem suporte ou sem gesto do usuário ainda: ignora */
  }
}

/** O usuário pediu menos animação no sistema. */
export function prefersReducedMotion(): boolean {
  return window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
}
