import { describe, expect, it } from 'vitest';

import type { Position } from '@/types';

import { parsePosition, parseRealtimeMessage } from './realtime';

const DEVICE = '0b9f6c1e-5a47-4f3e-9c1d-2e8f7a6b5c4d';
const VEHICLE = '7d1c2b3a-4e5f-4a6b-8c7d-9e0f1a2b3c4d';

const position: Position = {
  id: 42,
  deviceId: DEVICE,
  gpsTimestamp: '2026-09-30T12:00:00Z',
  receivedAt: '2026-09-30T12:00:01Z',
  latitude: -23.5505,
  longitude: -46.6333,
  speedKmh: 42.5,
  heading: 271.5,
  altitude: 760,
  gpsValid: true,
  satellites: 9,
  hdop: 0.9,
  acc: true,
  batteryVoltage: 4.1,
  batteryPercent: 88,
  gsmLevel: 4,
  relayOn: false,
  protocol: 'gt06',
  source: 'gps',
};

function message(type: string, data: unknown, extra: Record<string, unknown> = {}): string {
  return JSON.stringify({ type, deviceId: DEVICE, vehicleId: VEHICLE, timestamp: '2026-09-30T12:00:01Z', data, ...extra });
}

describe('parseRealtimeMessage', () => {
  it('posição legítima passa inteira', () => {
    const parsed = parseRealtimeMessage(message('position.updated', position));
    expect(parsed?.data).toEqual(position);
    expect(parsed?.vehicleId).toBe(VEHICLE);
  });

  it('rumo nulo é aceito', () => {
    expect(parsePosition({ ...position, heading: null })?.heading).toBeNull();
  });

  it.each([
    ['texto com markup', `0 16 16)" onload="alert(1)`],
    ['número em texto', '90'],
    ['objeto', { valueOf: 90 }],
    ['lista', [90]],
    ['booleano', true],
    ['acima de 360', 361],
    ['negativo', -0.1],
  ])('rejeita rumo %s', (_, heading) => {
    expect(parseRealtimeMessage(message('position.updated', { ...position, heading }))).toBeNull();
  });

  it('rejeita rumo não finito (1e999 vira Infinity no JSON.parse)', () => {
    const raw = message('position.updated', { ...position, heading: 1 }).replace('"heading":1', '"heading":1e999');
    expect(parseRealtimeMessage(raw)).toBeNull();
  });

  it.each([
    ['velocidade em texto', { speedKmh: '3' }],
    ['velocidade absurda', { speedKmh: 5000 }],
    ['latitude fora', { latitude: 91 }],
    ['longitude em texto', { longitude: '-46' }],
    ['ignição em texto', { acc: 'true' }],
    ['bateria em objeto', { batteryVoltage: {} }],
    ['aparelho não-uuid', { deviceId: '"><img src=x onerror=alert(1)>' }],
    ['origem inventada', { source: '<b>' }],
    ['protocolo com markup', { protocol: '<script>' }],
  ])('rejeita posição com %s', (_, patch) => {
    expect(parseRealtimeMessage(message('position.updated', { ...position, ...patch }))).toBeNull();
  });

  it('descarta campos que não pertencem à posição', () => {
    const parsed = parseRealtimeMessage(message('position.updated', { ...position, html: '<img onerror=alert(1)>' }));
    expect(parsed?.data).toEqual(position);
    expect(JSON.stringify(parsed)).not.toContain('onerror');
  });

  it.each([
    ['JSON quebrado', '{"type":'],
    ['tipo desconhecido', message('script.run', {})],
    ['sem timestamp', message('position.updated', position, { timestamp: 5 })],
    ['vehicleId não-uuid', message('position.updated', position, { vehicleId: '<x>' })],
    ['status inventado', message('device.online', { deviceId: DEVICE, status: '<b>ON</b>' })],
    ['relé em texto', message('engine.status.changed', { deviceId: DEVICE, relayOn: 'true' })],
    ['comando sem id', message('command.failed', { deviceId: DEVICE, status: 'FAILED' })],
    ['evento com tipo em markup', message('vehicle.event', { deviceId: DEVICE, type: '<svg/onload=1>' })],
    ['não é objeto', '"position.updated"'],
  ])('rejeita %s', (_, raw) => {
    expect(parseRealtimeMessage(raw)).toBeNull();
  });

  it('eventos legítimos dos outros tipos passam', () => {
    const cases: [string, unknown][] = [
      ['device.online', { deviceId: DEVICE, status: 'ONLINE' }],
      ['device.offline', { deviceId: DEVICE, status: 'OFFLINE', previous: 'STALE' }],
      ['device.online', { deviceId: DEVICE, state: { deviceId: DEVICE, acc: true } }],
      ['engine.status.changed', { deviceId: DEVICE, relayOn: true }],
      ['command.acknowledged', { id: VEHICLE, deviceId: DEVICE, status: 'ACKNOWLEDGED', command: 'ENGINE_STOP' }],
      ['vehicle.event', { id: 1, deviceId: DEVICE, type: 'OVERSPEED', metadata: {} }],
    ];
    for (const [type, data] of cases) {
      expect(parseRealtimeMessage(message(type, data)), type).not.toBeNull();
    }
  });
});
