import { defineConfig } from 'vite';
import type { Plugin } from 'vite';
import react from '@vitejs/plugin-react';
import { createHash } from 'node:crypto';
import { readFileSync, readdirSync } from 'node:fs';
import path from 'node:path';
import type { OutputBundle, OutputChunk } from 'rollup';

/**
 * O app do cliente (PWA) mora em /app: rotas como /app/veiculos/123 precisam
 * servir app/index.html (o fallback padrão do Vite mandaria para o painel).
 */
function appRoutes(): Plugin {
  const rewrite = (req: { url?: string }, _res: unknown, next: () => void) => {
    const pathname = (req.url ?? '').split('?')[0];
    if (/^\/app(\/|$)/.test(pathname) && !path.extname(pathname)) req.url = '/app/index.html';
    next();
  };
  return {
    name: 'farbo-app-routes',
    configureServer: (server) => void server.middlewares.use(rewrite),
    configurePreviewServer: (server) => void server.middlewares.use(rewrite),
  };
}

/**
 * Gera /app/sw.js no build: o modelo (src/app/sw-template.js) recebe a versão
 * e a lista exata de arquivos do app — só os do app, não os do painel.
 */
function pwaServiceWorker(): Plugin {
  return {
    name: 'farbo-pwa-service-worker',
    apply: 'build',
    generateBundle(_options, bundle: OutputBundle) {
      const chunks = Object.values(bundle).filter((f): f is OutputChunk => f.type === 'chunk');
      const entry = chunks.find((c) => c.isEntry && c.facadeModuleId?.endsWith(path.join('app', 'index.html')));
      if (!entry) this.error('entrada do app (app/index.html) não encontrada no build');

      const files = new Set<string>();
      const visit = (chunk: OutputChunk) => {
        if (files.has(chunk.fileName)) return;
        files.add(chunk.fileName);
        chunk.viteMetadata?.importedCss.forEach((css) => files.add(css));
        chunk.viteMetadata?.importedAssets.forEach((asset) => files.add(asset));
        chunk.imports.forEach((name) => {
          const next = bundle[name];
          if (next?.type === 'chunk') visit(next);
        });
      };
      visit(entry!);

      const icons = readdirSync(path.resolve(__dirname, 'public/app/icons')).map((f) => `/app/icons/${f}`);
      const brand = ['/assets/logo-header.png', '/assets/logo-mark.png'];
      const precache = ['/app/index.html', '/app/manifest.webmanifest', ...icons, ...brand, ...[...files].sort().map((f) => `/${f}`)];
      const version = createHash('sha256').update(precache.join('\n')).digest('hex').slice(0, 12);
      const source = readFileSync(path.resolve(__dirname, 'src/app/sw-template.js'), 'utf8')
        .replaceAll('__VERSION__', version)
        .replaceAll('__PRECACHE__', JSON.stringify(precache, null, 2));
      if (source.includes('__')) this.error('marcador não preenchido no sw.js');
      this.emitFile({ type: 'asset', fileName: 'app/sw.js', source });
    },
  };
}

export default defineConfig({
  plugins: [react(), appRoutes(), pwaServiceWorker()],
  resolve: {
    alias: { '@': path.resolve(__dirname, './src') },
  },
  server: {
    port: 5173,
    host: true,
    // O Vite recusa hosts desconhecidos (proteção contra DNS rebinding).
    // farbo.localtest.me aponta para 127.0.0.1 e é o endereço cadastrado no
    // Melhor Envios para o retorno do OAuth em desenvolvimento.
    allowedHosts: ['farbo.localtest.me'],
    proxy: {
      // Em desenvolvimento o Vite encaminha para o backend, evitando CORS.
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
  build: {
    outDir: 'dist',
    // Os mapas são gerados para depuração, mas o bundle não aponta para eles
    // e o nginx não os serve: o código-fonte original não fica público.
    sourcemap: 'hidden',
    rollupOptions: {
      // Duas entradas: o painel (/) e o app do cliente (/app/).
      input: {
        main: path.resolve(__dirname, 'index.html'),
        app: path.resolve(__dirname, 'app/index.html'),
      },
    },
  },
});
