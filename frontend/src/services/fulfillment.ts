import type { ChipStatus, FulfillmentEvent, FulfillmentTrack, TrackerStatus } from '@/types';
import type { BadgeTone } from '@/components/ui/Badge';

export interface Step<S extends string> {
  status: S;
  label: string;
  /** Rótulo curto para selos e tabelas. */
  short: string;
}

export const CHIP_STEPS: Step<ChipStatus>[] = [
  { status: 'REQUESTED', label: 'Chip solicitado no fornecedor', short: 'Solicitado' },
  { status: 'SHIPPED', label: 'Chip enviado', short: 'Enviado' },
  { status: 'AT_BASE', label: 'Chip chegou em nossa base', short: 'Na base' },
  { status: 'SEPARATED', label: 'Chip separado para configuração', short: 'Separado' },
];

export const TRACKER_STEPS: Step<TrackerStatus>[] = [
  { status: 'AWAITING_SUPPLIER', label: 'Aguardando chegada do rastreador pelo fornecedor', short: 'Aguardando fornecedor' },
  { status: 'AT_BASE', label: 'Rastreador chegou em nossa base', short: 'Na base' },
  { status: 'AWAITING_CHIP', label: 'Aguardando chegada do chip M2M', short: 'Aguardando chip' },
  { status: 'CONFIGURING', label: 'Rastreador em configuração', short: 'Em configuração' },
  { status: 'CONFIGURED', label: 'Rastreador configurado', short: 'Configurado' },
  { status: 'SHIPPED', label: 'Rastreador enviado', short: 'Enviado' },
  { status: 'IN_TRANSIT', label: 'Rastreador em trânsito', short: 'Em trânsito' },
  { status: 'DELIVERED', label: 'Rastreador chegou', short: 'Chegou' },
];

export function stepsOf(track: FulfillmentTrack): Step<string>[] {
  return track === 'CHIP' ? CHIP_STEPS : TRACKER_STEPS;
}

export function stepIndex(track: FulfillmentTrack, status: string): number {
  return stepsOf(track).findIndex((step) => step.status === status);
}

export function stepOf(track: FulfillmentTrack, status: string): Step<string> | undefined {
  return stepsOf(track).find((step) => step.status === status);
}

/** Quando a etapa foi alcançada pela última vez (o histórico guarda tudo). */
export function reachedAt(events: FulfillmentEvent[], track: FulfillmentTrack, status: string): string | undefined {
  for (let i = events.length - 1; i >= 0; i--) {
    if (events[i].track === track && events[i].status === status) return events[i].createdAt;
  }
  return undefined;
}

export function trackingUrl(code: string): string {
  return `https://www.melhorrastreio.com.br/rastreio/${encodeURIComponent(code)}`;
}

/** Cor do selo do rastreador: parado no fornecedor, na base, a caminho, chegou. */
export function trackerTone(status: TrackerStatus): BadgeTone {
  switch (status) {
    case 'DELIVERED':
      return 'success';
    case 'SHIPPED':
    case 'IN_TRANSIT':
      return 'accent';
    case 'AWAITING_CHIP':
      return 'warning';
    default:
      return 'neutral';
  }
}
