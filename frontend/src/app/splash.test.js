// Em JavaScript: usa fs e path do Node, que os tipos do projeto (navegador) não incluem.
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import sharp from 'sharp';
import { describe, expect, it } from 'vitest';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const html = readFileSync(path.join(root, 'app/index.html'), 'utf8');
const links = [...html.matchAll(/<link rel="apple-touch-startup-image" media="([^"]+)" href="([^"]+)"/g)];

describe('telas de abertura do iPhone', () => {
  it('existem para os iPhones atuais', () => {
    expect(links.length).toBeGreaterThanOrEqual(12);
  });

  it.each(links.map((m) => [m[2], m[1]]))('%s tem o tamanho exato do aparelho', async (href, media) => {
    const w = Number(/device-width: (\d+)px/.exec(media)?.[1]);
    const h = Number(/device-height: (\d+)px/.exec(media)?.[1]);
    const ratio = Number(/-webkit-device-pixel-ratio: (\d+)/.exec(media)?.[1]);
    const meta = await sharp(path.join(root, 'public', href)).metadata();
    expect([meta.width, meta.height]).toEqual([w * ratio, h * ratio]);
  });
});
