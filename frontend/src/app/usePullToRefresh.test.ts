import { describe, expect, it } from 'vitest';

import { PULL_THRESHOLD, rubberBand } from './usePullToRefresh';

describe('rubberBand', () => {
  it('não anda sem puxar e cresce com o dedo', () => {
    expect(rubberBand(0)).toBe(0);
    expect(rubberBand(-20)).toBe(0);
    expect(rubberBand(50)).toBeLessThan(rubberBand(100));
  });

  it('resiste: nunca passa de 120px, por mais que puxe', () => {
    expect(rubberBand(10_000)).toBeLessThanOrEqual(120);
  });

  it('dispara com um puxão de ~100px (como no iOS), não com um toque que escorrega', () => {
    expect(rubberBand(110)).toBeGreaterThanOrEqual(PULL_THRESHOLD);
    expect(rubberBand(60)).toBeLessThan(PULL_THRESHOLD);
  });
});
