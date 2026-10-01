/**
 * Conta da lista arrastável do mapa: em quais alturas ela para e para qual
 * vai quando o dedo solta.
 */

/** Altura visível mínima: a alça e o título. */
export const PEEK_EXTRA = 8;
/** A posição do meio não passa desta fração da área disponível. */
export const HALF_RATIO = 0.5;
/** Arrasto rápido (px/ms) vale como "jogar" a lista para a próxima parada. */
export const FLICK_VELOCITY = 0.45;

export interface SheetMeasures {
  /** Altura total que a lista pode ocupar (do topo livre até as abas). */
  available: number;
  /** Alça + título. */
  header: number;
  /** Altura de todo o conteúdo da lista. */
  content: number;
}

/**
 * Paradas em ordem crescente de altura visível: recolhida (só o título), meio
 * (o conteúdo inteiro, se couber na metade) e aberta (só quando o conteúdo
 * não cabe no meio).
 */
export function computeSnaps({ available, header, content }: SheetMeasures): number[] {
  const peek = Math.min(header + PEEK_EXTRA, available);
  const fit = header + content;
  const half = Math.max(peek, Math.min(fit, available * HALF_RATIO));
  const snaps = [peek];
  if (half - peek > 24) snaps.push(half);
  if (fit > half + 24) snaps.push(available);
  return snaps;
}

/** Parada escolhida ao soltar: a mais próxima, ou a seguinte se foi um "jogo". */
export function pickSnap(snaps: number[], visible: number, velocity: number): number {
  if (velocity <= -FLICK_VELOCITY) {
    const above = snaps.findIndex((snap) => snap > visible + 1);
    return above === -1 ? snaps.length - 1 : above;
  }
  if (velocity >= FLICK_VELOCITY) {
    for (let i = snaps.length - 1; i >= 0; i--) if (snaps[i] < visible - 1) return i;
    return 0;
  }
  let best = 0;
  snaps.forEach((snap, i) => {
    if (Math.abs(snap - visible) < Math.abs(snaps[best] - visible)) best = i;
  });
  return best;
}
