// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';

import { endpointIcon, historyIcon, safeHeading, vehicleIcon } from './markers';

/** O que o Leaflet põe no mapa: a div criada pelo próprio ícone. */
function render(icon: ReturnType<typeof vehicleIcon>): HTMLElement {
  return icon.createIcon() as HTMLElement;
}

const PAYLOADS = [
  `0 16 16)" onload="window.__pwned=1`,
  `"><img src=x onerror="window.__pwned=1">`,
  `</svg><script>window.__pwned=1</script>`,
  `90) scale(40`,
];

describe('vehicleIcon', () => {
  it('gira a seta pelo rumo numérico', () => {
    const el = render(vehicleIcon({ ignition: true, heading: 271.5, moving: true, selected: false, blocked: false }));
    expect(el.querySelector('path')?.getAttribute('transform')).toBe('rotate(271.5 16 16)');
  });

  it('texto com aspas ou marcação no rumo não cria nós nem atributos', () => {
    const clean = render(vehicleIcon({ ignition: true, heading: 0, moving: true, selected: true, blocked: true }));

    for (const payload of PAYLOADS) {
      const el = render(
        vehicleIcon({
          ignition: true,
          // Simula o que chegaria sem validação: o tipo mente, o valor é texto.
          heading: payload as unknown as number,
          moving: true,
          selected: true,
          blocked: true,
        }),
      );
      expect(el.outerHTML).toBe(clean.outerHTML);
      expect(el.querySelectorAll('script, img, foreignObject')).toHaveLength(0);
      for (const node of el.querySelectorAll('*')) {
        for (const attribute of node.getAttributeNames()) {
          expect(attribute.startsWith('on')).toBe(false);
        }
      }
    }
    expect((window as unknown as { __pwned?: number }).__pwned).toBeUndefined();
  });

  it('valores fora da faixa viram rumo zero', () => {
    for (const heading of [Number.NaN, Infinity, -1, 361, { toString: () => '45' }, [90], null]) {
      expect(safeHeading(heading)).toBe(0);
    }
    expect(safeHeading(360)).toBe(360);
    expect(safeHeading(12.5)).toBe(12.5);
  });

  it('mantém o desenho de cada estado', () => {
    const stopped = render(vehicleIcon({ ignition: false, heading: 10, moving: false, selected: false, blocked: false }));
    expect(stopped.querySelector('circle')?.getAttribute('fill')).toBe('#ef5b52');
    expect(stopped.querySelector('path')).toBeNull();

    const offline = render(
      vehicleIcon({ ignition: null, heading: null, moving: true, selected: false, blocked: true, online: false }),
    );
    const svg = offline.querySelector('svg');
    expect(svg?.getAttribute('opacity')).toBe('0.55');
    expect(svg?.getAttribute('viewBox')).toBe('0 0 32 46');
    expect(offline.querySelector('text')?.textContent).toBe('🔒');
    expect(offline.querySelector('path')?.getAttribute('fill')).toBe('#64748b');
  });

  it('cada chamada devolve um desenho novo (o Leaflet move o nó para o marcador)', () => {
    const options = { ignition: true, heading: 5, moving: true, selected: false, blocked: false };
    expect(render(vehicleIcon(options)).firstChild).not.toBe(render(vehicleIcon(options)).firstChild);
  });
});

describe('ícones auxiliares', () => {
  it('não aceitam marcação pela cor', () => {
    const el = render(historyIcon(`red" onload="window.__pwned=1`));
    expect(el.querySelectorAll('*')).toHaveLength(2);
    expect(el.querySelector('circle')?.getAttributeNames()).not.toContain('onload');
    expect(render(endpointIcon('start')).querySelector('circle')?.getAttribute('fill')).toBe('#3be558');
  });
});
