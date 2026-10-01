// Gera os ícones do app do cliente (PWA) a partir da marca. Rode de novo se
// a logo mudar: node scripts/pwa-icons.mjs
import sharp from 'sharp';

const SOURCE = 'public/assets/logo-mark.png';
const OUT = 'public/app/icons';
const BACKGROUND = { r: 6, g: 9, b: 7, alpha: 1 }; // #060907, o fundo da marca

/** Logo centralizada ocupando `ratio` do lado, sobre o fundo escuro. */
async function icon(size, ratio, file) {
  const inner = Math.round(size * ratio);
  const logo = await sharp(SOURCE).resize(inner, inner, { fit: 'inside' }).toBuffer();
  await sharp({ create: { width: size, height: size, channels: 4, background: BACKGROUND } })
    .composite([{ input: logo, gravity: 'center' }])
    .png()
    .toFile(`${OUT}/${file}`);
}

// Comum: a logo ocupa 70% do quadrado.
await icon(192, 0.7, 'icon-192.png');
await icon(512, 0.7, 'icon-512.png');
// Maskable: o Android recorta até 20% das bordas; a logo fica nos 56% do meio.
await icon(512, 0.56, 'maskable-512.png');
// iPhone (Tela de Início): sem transparência.
await icon(180, 0.66, 'apple-touch-icon.png');

// Badge da barra de notificações do Android: silhueta branca, fundo transparente.
const size = 96;
const { data, info } = await sharp(SOURCE)
  .resize(Math.round(size * 0.84), Math.round(size * 0.84), { fit: 'inside' })
  .ensureAlpha()
  .extractChannel('alpha')
  .raw()
  .toBuffer({ resolveWithObject: true });
const white = await sharp({ create: { width: info.width, height: info.height, channels: 3, background: '#ffffff' } })
  .joinChannel(data, { raw: { width: info.width, height: info.height, channels: 1 } })
  .png()
  .toBuffer();
await sharp({ create: { width: size, height: size, channels: 4, background: { r: 0, g: 0, b: 0, alpha: 0 } } })
  .composite([{ input: white, gravity: 'center' }])
  .png()
  .toFile(`${OUT}/badge-96.png`);

console.log('ícones gerados em', OUT);

// Logo menor para o cabeçalho e o rodapé do site: a original tem 956px de
// largura (e ~110 KB) para aparecer com ~150px. Com srcset, o celular baixa
// esta; telas maiores e os e-mails continuam com a original.
await sharp('public/assets/logo-header.png')
  .resize(480)
  .png({ compressionLevel: 9, effort: 10 })
  .toFile('public/assets/logo-header-480.png');

// Telas de abertura do iPhone (apple-touch-startup-image). Sem elas o iOS abre
// o app instalado numa tela branca. Uma por tamanho de tela, em pontos e
// densidade; os <link> correspondentes estão em app/index.html.
const SPLASH_OUT = 'public/app/splash';
const SPLASH = [
  [440, 956, 3], // iPhone 16/17 Pro Max
  [420, 912, 3], // iPhone Air
  [402, 874, 3], // iPhone 16/17 Pro, iPhone 17
  [430, 932, 3], // iPhone 14/15 Pro Max, 15/16 Plus
  [393, 852, 3], // iPhone 14/15 Pro, 15, 16
  [428, 926, 3], // iPhone 12/13 Pro Max, 14 Plus
  [390, 844, 3], // iPhone 12, 13, 14, 16e
  [375, 812, 3], // iPhone X, XS, 11 Pro, 12/13 mini
  [414, 896, 3], // iPhone XS Max, 11 Pro Max
  [414, 896, 2], // iPhone XR, 11
  [414, 736, 3], // iPhone 8 Plus
  [375, 667, 2], // iPhone 8, SE (2ª e 3ª geração)
];
const { mkdirSync } = await import('node:fs');
mkdirSync(SPLASH_OUT, { recursive: true });
const links = [];
for (const [w, h, ratio] of SPLASH) {
  const width = w * ratio;
  const height = h * ratio;
  const logo = await sharp('public/assets/logo-header.png').resize(Math.round(width * 0.58)).toBuffer();
  const file = `splash-${width}x${height}.png`;
  await sharp({ create: { width, height, channels: 4, background: BACKGROUND } })
    .composite([{ input: logo, gravity: 'center' }])
    .png({ palette: true, quality: 90, effort: 10 })
    .toFile(`${SPLASH_OUT}/${file}`);
  links.push(
    `    <link rel="apple-touch-startup-image" media="screen and (device-width: ${w}px) and (device-height: ${h}px) and (-webkit-device-pixel-ratio: ${ratio}) and (orientation: portrait)" href="/app/splash/${file}" />`,
  );
}
console.log('telas de abertura geradas em', SPLASH_OUT);
if (process.argv.includes('--links')) console.log(links.join('\n'));
