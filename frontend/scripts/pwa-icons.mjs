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
