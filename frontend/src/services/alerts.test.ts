import { describe, expect, it } from 'vitest';

import type { AlertKindInfo } from '@/types';

import { alertKindLabel, isClock, sameKinds } from './alerts';

const catalog: AlertKindInfo[] = [
  { kind: 'SOS', label: 'Botão de pânico (SOS)', description: '', security: true, default: true },
  { kind: 'IGNITION_GUARD', label: 'Ignição ligada no horário de vigilância', description: '', security: false, default: true },
];

describe('alertKindLabel', () => {
  it('usa o nome do catálogo do servidor', () => {
    expect(alertKindLabel('SOS', catalog)).toBe('Botão de pânico (SOS)');
  });

  it('conhece os tipos que só existem no histórico', () => {
    expect(alertKindLabel('ENGINE_BLOCKED', catalog)).toBe('Motor bloqueado');
    expect(alertKindLabel('ENGINE_UNBLOCKED', catalog)).toBe('Motor liberado');
    expect(alertKindLabel('TEST', catalog)).toBe('E-mail de teste');
  });

  it('tipo desconhecido aparece como veio', () => {
    expect(alertKindLabel('NOVO_TIPO', catalog)).toBe('NOVO_TIPO');
  });
});

describe('isClock', () => {
  it.each(['00:00', '06:00', '22:30', '23:59', '7:15'])('aceita %s', (value) => {
    expect(isClock(value)).toBe(true);
  });
  it.each(['24:00', '22:60', '22h', '', '2230', '22:0'])('recusa %s', (value) => {
    expect(isClock(value)).toBe(false);
  });
});

describe('sameKinds', () => {
  it('ignora a ordem', () => {
    expect(sameKinds(['SOS', 'TOWING'], ['TOWING', 'SOS'])).toBe(true);
    expect(sameKinds(['SOS'], ['SOS', 'TOWING'])).toBe(false);
  });
});
