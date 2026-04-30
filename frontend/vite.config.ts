import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { VitePWA } from 'vite-plugin-pwa';
import path from 'node:path';

// The Go backend (internal/pkg/rest/static.go) serves the SPA out of `/static`,
// COPYed straight from `frontend/build/` in the multi-stage Dockerfile. Keep
// `build.outDir = 'build'` to avoid touching the Dockerfile when migrating
// from CRA to Vite. Do not "fix" this to `dist`.
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: 'autoUpdate',
      injectRegister: 'auto',
      includeAssets: ['favicon.ico', 'robots.txt', 'logo192.png', 'logo512.png'],
      manifest: {
        name: 'Lazy Soap',
        short_name: 'Lazy Soap',
        description: 'Find the best episodes of TV series.',
        theme_color: '#414141',
        background_color: '#414141',
        display: 'standalone',
        start_url: '/',
        icons: [
          { src: 'logo192.png', sizes: '192x192', type: 'image/png' },
          { src: 'logo512.png', sizes: '512x512', type: 'image/png' },
          { src: 'logo512.png', sizes: '512x512', type: 'image/png', purpose: 'any maskable' },
        ],
      },
      workbox: {
        // Avoid caching API responses; React Query owns runtime caching.
        navigateFallbackDenylist: [/^\/(search|id|img|featured|meta|metrics|debug|ping|version)/],
        globPatterns: ['**/*.{js,css,html,ico,png,svg,webp,woff2}'],
      },
    }),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  build: {
    outDir: 'build',
    sourcemap: true,
    target: 'es2022',
    cssCodeSplit: true,
    rollupOptions: {
      output: {
        manualChunks: {
          react: ['react', 'react-dom'],
          query: ['@tanstack/react-query'],
        },
      },
    },
  },
  server: {
    port: 5173,
    strictPort: false,
    proxy: {
      '/search': { target: 'http://127.0.0.1:8202', changeOrigin: true },
      '/id': { target: 'http://127.0.0.1:8202', changeOrigin: true },
      '/img': { target: 'http://127.0.0.1:8202', changeOrigin: true },
      '/featured': { target: 'http://127.0.0.1:8202', changeOrigin: true },
      '/meta': { target: 'http://127.0.0.1:8202', changeOrigin: true },
      '/ping': { target: 'http://127.0.0.1:8202', changeOrigin: true },
      '/version': { target: 'http://127.0.0.1:8202', changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./vitest.setup.ts'],
    css: true,
    exclude: ['node_modules', 'build', 'dist', 'e2e', 'playwright-report'],
  },
});
