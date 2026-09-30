import type { Position, RealtimeEventType, RealtimeMessage } from '@/types';

/**
 * Conferência dos eventos do WebSocket.
 *
 * Os tipos do TypeScript somem na compilação: o que chega pelo socket é texto
 * qualquer. Antes de ir para o cache (e dele para o mapa), cada evento é
 * conferido campo a campo e refeito só com os campos conhecidos. Campo com
 * tipo errado ou valor fora da faixa descarta o evento inteiro — o próximo
 * refetch da lista corrige o que ficou para trás.
 */

const EVENT_TYPES = new Set<RealtimeEventType>([
  'position.updated',
  'device.online',
  'device.offline',
  'device.stale',
  'vehicle.event',
  'command.sent',
  'command.acknowledged',
  'command.failed',
  'engine.status.changed',
]);

const DEVICE_STATUSES = new Set(['ONLINE', 'STALE', 'OFFLINE']);
const COMMAND_STATUSES = new Set([
  'PENDING',
  'SENDING',
  'SENT',
  'ACKNOWLEDGED',
  'FAILED',
  'TIMEOUT',
  'REJECTED',
]);
const SOURCES = new Set<Position['source']>(['gps', 'heartbeat', 'lbs']);

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const PROTOCOL = /^[a-z0-9_-]{0,32}$/;
const EVENT_KIND = /^[A-Z0-9_]{1,64}$/;

/** Velocidade acima disso não é de veículo terrestre: é lixo. */
const MAX_SPEED_KMH = 1000;

type Json = Record<string, unknown>;

function isRecord(value: unknown): value is Json {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isUuid(value: unknown): value is string {
  return typeof value === 'string' && UUID.test(value);
}

function isNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

function inRange(value: unknown, min: number, max: number): value is number {
  return isNumber(value) && value >= min && value <= max;
}

/** null/ausente viram null; número finito passa; o resto é inválido. */
function optionalNumber(value: unknown): number | null | undefined {
  if (value === null || value === undefined) return null;
  return isNumber(value) ? value : undefined;
}

function optionalBoolean(value: unknown): boolean | null | undefined {
  if (value === null || value === undefined) return null;
  return typeof value === 'boolean' ? value : undefined;
}

/** Rumo válido: número de 0 a 360 graus (ou ausente). */
export function isValidHeading(value: unknown): value is number | null | undefined {
  return value === null || value === undefined || inRange(value, 0, 360);
}

export function parsePosition(data: unknown): Position | null {
  if (!isRecord(data)) return null;
  const { deviceId, latitude, longitude, speedKmh, heading, protocol, source } = data;
  if (!isUuid(deviceId)) return null;
  if (!inRange(latitude, -90, 90) || !inRange(longitude, -180, 180)) return null;
  if (!inRange(speedKmh, 0, MAX_SPEED_KMH) || !isValidHeading(heading)) return null;
  if (typeof data.gpsTimestamp !== 'string' || typeof data.receivedAt !== 'string') return null;
  if (typeof protocol !== 'string' || !PROTOCOL.test(protocol)) return null;
  if (!SOURCES.has(source as Position['source'])) return null;
  if (!isNumber(data.id)) return null;

  const numbers = {
    altitude: optionalNumber(data.altitude),
    satellites: optionalNumber(data.satellites),
    hdop: optionalNumber(data.hdop),
    batteryVoltage: optionalNumber(data.batteryVoltage),
    batteryPercent: optionalNumber(data.batteryPercent),
    gsmLevel: optionalNumber(data.gsmLevel),
  };
  const flags = {
    gpsValid: optionalBoolean(data.gpsValid),
    acc: optionalBoolean(data.acc),
    relayOn: optionalBoolean(data.relayOn),
  };
  if (Object.values(numbers).includes(undefined) || Object.values(flags).includes(undefined)) {
    return null;
  }

  return {
    id: data.id,
    deviceId,
    gpsTimestamp: data.gpsTimestamp,
    receivedAt: data.receivedAt,
    latitude,
    longitude,
    speedKmh,
    heading: heading ?? null,
    protocol,
    source: source as Position['source'],
    ...(numbers as { [K in keyof typeof numbers]: number | null }),
    ...(flags as { [K in keyof typeof flags]: boolean | null }),
  };
}

function parseDeviceStatus(data: unknown): Json | null {
  if (!isRecord(data)) return null;
  if (data.deviceId !== undefined && !isUuid(data.deviceId)) return null;
  for (const key of ['status', 'previous'] as const) {
    if (data[key] !== undefined && !DEVICE_STATUSES.has(data[key] as string)) return null;
  }
  if (data.state !== undefined && !isRecord(data.state)) return null;
  return { deviceId: data.deviceId, status: data.status, previous: data.previous, state: data.state };
}

function parseEngineStatus(data: unknown): Json | null {
  if (!isRecord(data) || !isUuid(data.deviceId) || typeof data.relayOn !== 'boolean') return null;
  return { deviceId: data.deviceId, relayOn: data.relayOn };
}

function parseCommand(data: unknown): Json | null {
  if (!isRecord(data) || !isUuid(data.id) || !isUuid(data.deviceId)) return null;
  if (!COMMAND_STATUSES.has(data.status as string)) return null;
  return data;
}

function parseVehicleEvent(data: unknown): Json | null {
  if (!isRecord(data) || !isUuid(data.deviceId)) return null;
  if (typeof data.type !== 'string' || !EVENT_KIND.test(data.type)) return null;
  return data;
}

function parseData(type: RealtimeEventType, data: unknown): unknown {
  switch (type) {
    case 'position.updated':
      return parsePosition(data);
    case 'device.online':
    case 'device.offline':
    case 'device.stale':
      return data === undefined ? undefined : parseDeviceStatus(data);
    case 'engine.status.changed':
      return parseEngineStatus(data);
    case 'command.sent':
    case 'command.acknowledged':
    case 'command.failed':
      return parseCommand(data);
    case 'vehicle.event':
      return parseVehicleEvent(data);
  }
}

/** Confere uma mensagem do socket; null quando ela deve ser ignorada. */
export function parseRealtimeMessage(raw: string): RealtimeMessage | null {
  let message: unknown;
  try {
    message = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!isRecord(message) || !EVENT_TYPES.has(message.type as RealtimeEventType)) return null;
  if (typeof message.timestamp !== 'string') return null;
  if (message.vehicleId !== undefined && !isUuid(message.vehicleId)) return null;
  if (message.deviceId !== undefined && !isUuid(message.deviceId)) return null;

  const type = message.type as RealtimeEventType;
  const data = parseData(type, message.data);
  if (data === null) return null;

  return {
    type,
    vehicleId: message.vehicleId,
    deviceId: message.deviceId,
    timestamp: message.timestamp,
    data,
  };
}
