// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';

import { isIos, pushSupport, urlBase64ToUint8Array } from './pwa';

describe('urlBase64ToUint8Array', () => {
  it('converte a applicationServerKey (base64url sem padding) em bytes', () => {
    // Chave pública do exemplo da RFC 8291: ponto P-256 não comprimido.
    const bytes = urlBase64ToUint8Array('BP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A8');
    expect(bytes.length).toBe(65);
    expect(bytes[0]).toBe(0x04);
    expect(bytes[64]).toBe(0x0f);
  });

  it('aceita os caracteres - e _ do base64url', () => {
    expect([...urlBase64ToUint8Array('-_-_')]).toEqual([0xfb, 0xff, 0xbf]);
  });
});

describe('isIos', () => {
  it.each([
    ['Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15', 5, true],
    ['Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari', 5, true], // iPad
    ['Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari', 0, false], // Mac
    ['Mozilla/5.0 (Linux; Android 14; Pixel 8) Chrome/128', 5, false],
  ])('%s → %s', (ua, touch, expected) => {
    expect(isIos(ua, touch as number)).toBe(expected);
  });
});

describe('pushSupport', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('sem PushManager nem Notification: não suportado', () => {
    expect(pushSupport()).toBe('unsupported');
  });

  it('com service worker, PushManager e Notification: suportado', () => {
    vi.stubGlobal('PushManager', class {});
    vi.stubGlobal('Notification', class {});
    Object.defineProperty(navigator, 'serviceWorker', { value: {}, configurable: true });
    expect(pushSupport()).toBe('supported');
  });
});
