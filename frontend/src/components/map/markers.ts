import L from 'leaflet';

/**
 * Ícones do mapa.
 *
 * O marcador é um SVG inline em vez de imagem: assim ele acompanha os tokens
 * de cor do tema e gira conforme o rumo sem precisar de sprite por ângulo.
 *
 * O SVG é montado nó a nó (createElementNS + setAttribute), nunca como texto
 * HTML: o rumo e os demais valores vêm do rastreador pelo WebSocket, e texto
 * interpolado num innerHTML viraria injeção de marcação. Com setAttribute um
 * valor estranho fica, no máximo, um atributo inválido — nunca um nó novo.
 */

const SVG_NS = 'http://www.w3.org/2000/svg';

/** Verde = ignição ligada, vermelho = desligada, cinza = ainda sem leitura. */
const IGNITION_ON = '#3be558';
const IGNITION_OFF = '#ef5b52';
const IGNITION_UNKNOWN = '#64748b';

/** Contorno dos ícones: o fundo da marca, para destacá-los sobre o mapa. */
const OUTLINE = '#060907';

/** Cor do selo de bloqueio, deliberadamente distinta do vermelho de ignição
 * desligada: as duas coisas podem ocorrer juntas e precisam ser distinguíveis
 * à primeira vista (ver docs sobre ACC × relé serem sinais independentes). */
const BLOCKED_BADGE = '#b3261e';

/** Altura reservada ao selo de bloqueio, na mesma unidade do desenho do
 * veículo (que usa um viewBox fixo de 32 unidades de largura). */
const BADGE_BAND = 14;

interface VehicleIconOptions {
  /**
   * Estado da ignição (ACC), lido do próprio rastreador.
   * true = ligada (verde) · false = desligada (vermelho) · null = sem leitura
   * ainda (cinza).
   */
  ignition: boolean | null;
  heading: number | null;
  /** Veículo parado ganha um círculo; em movimento, uma seta. */
  moving: boolean;
  selected: boolean;
  /**
   * Relé de corte acionado. Mostra um selo de cadeado acima do marcador — não
   * muda a cor do marcador em si, porque ignição e bloqueio são dois sinais
   * independentes do rastreador (o motor pode estar bloqueado com a ignição
   * ligada ou desligada).
   */
  blocked: boolean;
  /**
   * Dispositivo sem comunicação recente (STALE/OFFLINE): o ícone fica
   * esmaecido para avisar que a cor pode não refletir o estado atual.
   */
  online?: boolean;
}

/** Rumo em graus, sempre um número de 0 a 360. Qualquer outra coisa (texto,
 * objeto, NaN, fora da faixa) vira 0: a seta aponta para o norte. */
export function safeHeading(heading: unknown): number {
  return typeof heading === 'number' && Number.isFinite(heading) && heading >= 0 && heading <= 360
    ? heading
    : 0;
}

type Attributes = Record<string, string | number>;

function svgNode<K extends keyof SVGElementTagNameMap>(
  tag: K,
  attributes: Attributes,
  ...children: SVGElement[]
): SVGElementTagNameMap[K] {
  const node = document.createElementNS(SVG_NS, tag);
  for (const [name, value] of Object.entries(attributes)) {
    node.setAttribute(name, String(value));
  }
  node.append(...children);
  return node;
}

/** O Leaflet aceita qualquer Element em `html` (confere com instanceof
 * Element e usa appendChild); a tipagem dele é que só fala em HTMLElement. */
function asIconHtml(node: SVGSVGElement): HTMLElement {
  return node as unknown as HTMLElement;
}

export function vehicleIcon({
  ignition,
  heading,
  moving,
  selected,
  blocked,
  online = true,
}: VehicleIconOptions): L.DivIcon {
  const color = ignition === true ? IGNITION_ON : ignition === false ? IGNITION_OFF : IGNITION_UNKNOWN;
  const rotation = safeHeading(heading);
  const size = selected ? 38 : 32;

  // O veículo é sempre desenhado no mesmo espaço de 32x32 unidades; quando há
  // selo de bloqueio, o viewBox cresce para cima e o grupo do veículo desce
  // pelo tamanho da faixa reservada — o desenho do veículo em si não muda.
  const bandUnits = blocked ? BADGE_BAND : 0;
  const totalUnits = 32 + bandUnits;
  const scale = size / 32;
  const pixelWidth = size;
  const pixelHeight = scale * totalUnits;

  const vehicle = svgNode('g', { transform: `translate(0, ${bandUnits})` });
  if (selected) {
    vehicle.append(
      svgNode('circle', { cx: 16, cy: 16, r: 15, fill: 'none', stroke: color, 'stroke-width': 1.5, opacity: 0.5 }),
    );
  }
  vehicle.append(
    moving
      ? svgNode('path', {
          d: 'M16 5 L23 25 L16 20.5 L9 25 Z',
          fill: color,
          stroke: OUTLINE,
          'stroke-width': 1.5,
          'stroke-linejoin': 'round',
          transform: `rotate(${rotation} 16 16)`,
        })
      : svgNode('circle', { cx: 16, cy: 16, r: 7.5, fill: color, stroke: OUTLINE, 'stroke-width': 2 }),
  );

  const svg = svgNode('svg', { width: pixelWidth, height: pixelHeight, viewBox: `0 0 32 ${totalUnits}` });
  if (!online) svg.setAttribute('opacity', '0.55');
  if (blocked) svg.append(...blockedBadge(16, BADGE_BAND / 2));
  svg.append(vehicle);

  // O ponto de ancoragem fica sempre no centro do veículo (nunca no selo),
  // para o marcador continuar exatamente sobre a coordenada do GPS.
  const anchorY = scale * (16 + bandUnits);

  return L.divIcon({
    className: 'vehicle-marker',
    html: asIconHtml(svg),
    iconSize: [pixelWidth, pixelHeight],
    iconAnchor: [pixelWidth / 2, anchorY],
    // Abre acima de tudo o que está desenhado no topo do ícone — o selo,
    // quando existe, ou o próprio veículo quando não há bloqueio.
    popupAnchor: [0, -anchorY],
  });
}

/** Selo de motor bloqueado: halo + círculo + cadeado, para chamar atenção
 * mesmo num ícone pequeno no mapa. */
function blockedBadge(cx: number, cy: number): SVGElement[] {
  const lock = svgNode('text', { x: cx, y: cy + 2.5, 'font-size': 7, 'text-anchor': 'middle' });
  lock.textContent = '🔒';
  return [
    svgNode('circle', { cx, cy, r: 8, fill: BLOCKED_BADGE, opacity: 0.25 }),
    svgNode('circle', { cx, cy, r: 6, fill: BLOCKED_BADGE, stroke: '#ffffff', 'stroke-width': 1.25 }),
    lock,
  ];
}

function dotIcon(className: string, size: number, radius: number, color: string, strokeWidth: number): L.DivIcon {
  const center = size / 2;
  return L.divIcon({
    className,
    html: asIconHtml(
      svgNode(
        'svg',
        { width: size, height: size, viewBox: `0 0 ${size} ${size}` },
        svgNode('circle', { cx: center, cy: center, r: radius, fill: color, stroke: OUTLINE, 'stroke-width': strokeWidth }),
      ),
    ),
    iconSize: [size, size],
    iconAnchor: [center, center],
  });
}

/** Marcador pequeno para os pontos do histórico. */
export function historyIcon(color = IGNITION_ON): L.DivIcon {
  return dotIcon('history-marker', 10, 3.5, color, 1.5);
}

/** Bandeiras de início e fim de um trajeto. */
export function endpointIcon(kind: 'start' | 'end'): L.DivIcon {
  return dotIcon('endpoint-marker', 18, 7, kind === 'start' ? IGNITION_ON : IGNITION_OFF, 2);
}
