import type { Position } from '@/types';

/** Distância em metros entre dois pontos (haversine). */
export function distanceMeters(a: { latitude: number; longitude: number }, b: { latitude: number; longitude: number }): number {
  const rad = Math.PI / 180;
  const dLat = (b.latitude - a.latitude) * rad;
  const dLon = (b.longitude - a.longitude) * rad;
  const h =
    Math.sin(dLat / 2) ** 2 + Math.cos(a.latitude * rad) * Math.cos(b.latitude * rad) * Math.sin(dLon / 2) ** 2;
  return 2 * 6371000 * Math.asin(Math.min(1, Math.sqrt(h)));
}

export interface TripSummary {
  distanceMeters: number;
  maxSpeedKmh: number;
  points: number;
}

/**
 * Resumo do trajeto. Saltos de GPS parado não contam como distância: um
 * trecho só soma quando o veículo andava (velocidade) ou o salto é grande
 * demais para ser ruído.
 */
export function summarizeTrip(track: Position[]): TripSummary {
  let distance = 0;
  let maxSpeed = 0;
  for (let i = 0; i < track.length; i++) {
    maxSpeed = Math.max(maxSpeed, track[i].speedKmh ?? 0);
    if (i === 0) continue;
    const step = distanceMeters(track[i - 1], track[i]);
    const moving = (track[i].speedKmh ?? 0) > 3 || (track[i - 1].speedKmh ?? 0) > 3;
    if (moving || step > 50) distance += step;
  }
  return { distanceMeters: distance, maxSpeedKmh: maxSpeed, points: track.length };
}

export type TripRange = 'today' | 'yesterday' | 'last24h';

/** Janela de tempo de cada opção, no horário local do aparelho. */
export function tripWindow(range: TripRange, now = new Date()): { from: string; to: string } {
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  switch (range) {
    case 'today':
      return { from: startOfToday.toISOString(), to: now.toISOString() };
    case 'yesterday': {
      const start = new Date(startOfToday);
      start.setDate(start.getDate() - 1);
      return { from: start.toISOString(), to: startOfToday.toISOString() };
    }
    case 'last24h':
      return { from: new Date(now.getTime() - 24 * 3600 * 1000).toISOString(), to: now.toISOString() };
  }
}

/** Link de rota até o veículo (abre o app de mapas do celular). */
export function directionsUrl(lat: number, lon: number): string {
  return `https://www.google.com/maps/dir/?api=1&destination=${lat.toFixed(6)},${lon.toFixed(6)}`;
}
