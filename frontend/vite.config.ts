import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';

export default defineConfig({
  plugins: [react()],
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
  },
});
