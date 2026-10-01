import { describe, expect, it } from 'vitest';

import { clientAreaHref } from './ClientAreaLink';

describe('clientAreaHref', () => {
  it('no celular a Área do cliente abre o app', () => {
    expect(clientAreaHref(true)).toBe('/app/');
  });

  it('no computador, o login do painel', () => {
    expect(clientAreaHref(false)).toBe('/login');
  });
});
