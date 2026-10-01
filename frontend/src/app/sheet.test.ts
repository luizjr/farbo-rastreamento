import { describe, expect, it } from 'vitest';

import { computeSnaps, pickSnap } from './sheet';

describe('computeSnaps', () => {
  it('poucos veículos: o meio mostra a lista inteira e não há posição "aberta"', () => {
    // 700px disponíveis, título de 70px, dois cartões (180px).
    expect(computeSnaps({ available: 700, header: 70, content: 180 })).toEqual([78, 250]);
  });

  it('lista longa: recolhida, meio (metade da tela) e aberta', () => {
    expect(computeSnaps({ available: 700, header: 70, content: 900 })).toEqual([78, 350, 700]);
  });

  it('lista vazia ou minúscula: só a posição recolhida', () => {
    expect(computeSnaps({ available: 700, header: 70, content: 10 })).toEqual([78]);
  });
});

describe('pickSnap', () => {
  const snaps = [78, 350, 700];

  it('solta devagar: a parada mais próxima', () => {
    expect(pickSnap(snaps, 300, 0)).toBe(1);
    expect(pickSnap(snaps, 600, 0.1)).toBe(2);
    expect(pickSnap(snaps, 120, -0.1)).toBe(0);
  });

  it('jogada para cima: a próxima parada acima, mesmo perto da atual', () => {
    expect(pickSnap(snaps, 360, -0.8)).toBe(2);
    expect(pickSnap(snaps, 90, -0.8)).toBe(1);
    expect(pickSnap(snaps, 700, -0.8)).toBe(2);
  });

  it('jogada para baixo: a próxima parada abaixo', () => {
    expect(pickSnap(snaps, 340, 0.8)).toBe(0);
    expect(pickSnap(snaps, 690, 0.8)).toBe(1);
    expect(pickSnap(snaps, 78, 0.8)).toBe(0);
  });
});
