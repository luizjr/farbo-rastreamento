import { describe, expect, it } from 'vitest';

import type { Position } from '@/types';

import { directionsUrl, distanceMeters, summarizeTrip, tripWindow } from './trip';

const at = (latitude: number, longitude: number, speedKmh: number): Position =>
  ({ latitude, longitude, speedKmh }) as Position;

describe('summarizeTrip', () => {
  it('soma os trechos em movimento e guarda a velocidade máxima', () => {
    const track = [at(-23.55, -46.63, 0), at(-23.54, -46.63, 40), at(-23.53, -46.63, 62)];
    const summary = summarizeTrip(track);
    expect(summary.distanceMeters).toBeGreaterThan(2200);
    expect(summary.distanceMeters).toBeLessThan(2250);
    expect(summary.maxSpeedKmh).toBe(62);
    expect(summary.points).toBe(3);
  });

  it('ruído de GPS parado não vira distância', () => {
    const parked = [at(-23.55, -46.63, 0), at(-23.55001, -46.63002, 0), at(-23.54998, -46.62999, 0)];
    expect(summarizeTrip(parked).distanceMeters).toBe(0);
  });

  it('trajeto vazio', () => {
    expect(summarizeTrip([])).toEqual({ distanceMeters: 0, maxSpeedKmh: 0, points: 0 });
  });
});

describe('tripWindow', () => {
  const now = new Date(2026, 8, 30, 15, 30); // 30/09 15:30 no fuso local

  it('hoje: da meia-noite até agora', () => {
    const w = tripWindow('today', now);
    expect(new Date(w.from)).toEqual(new Date(2026, 8, 30, 0, 0));
    expect(new Date(w.to)).toEqual(now);
  });

  it('ontem: o dia inteiro anterior', () => {
    const w = tripWindow('yesterday', now);
    expect(new Date(w.from)).toEqual(new Date(2026, 8, 29, 0, 0));
    expect(new Date(w.to)).toEqual(new Date(2026, 8, 30, 0, 0));
  });

  it('últimas 24 h', () => {
    const w = tripWindow('last24h', now);
    expect(new Date(w.to).getTime() - new Date(w.from).getTime()).toBe(24 * 3600 * 1000);
  });
});

it('distância conhecida (~111 km por grau de latitude)', () => {
  expect(distanceMeters(at(0, 0, 0), at(1, 0, 0))).toBeGreaterThan(111000);
  expect(distanceMeters(at(0, 0, 0), at(1, 0, 0))).toBeLessThan(111300);
});

it('link de rota até o veículo', () => {
  expect(directionsUrl(-23.5505123, -46.6333789)).toBe(
    'https://www.google.com/maps/dir/?api=1&destination=-23.550512,-46.633379',
  );
});
