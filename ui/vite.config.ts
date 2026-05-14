import { defineConfig } from 'vite';
import solidPlugin from 'vite-plugin-solid';
// import devtools from 'solid-devtools/vite';

export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    include: ['src/**/*.test.{ts,tsx}'],
    setupFiles: [new URL('./src/test/mock-jest-dom.js', import.meta.url).pathname],
  },
  resolve: {
    alias: [
      { find: /@testing-library\/jest-dom(.*)/, replacement: new URL('./src/test/mock-jest-dom.js', import.meta.url).pathname },
      { find: new URL(process.env.HOME || '', import.meta.url).pathname + '/node_modules/@testing-library/jest-dom/dist/vitest.mjs', replacement: new URL('./src/test/mock-jest-dom.js', import.meta.url).pathname },
    ],
  },
  base: '/',
  plugins: [
    /* 
    Uncomment the following line to enable solid-devtools.
    For more info see https://github.com/thetarnav/solid-devtools/tree/main/packages/extension#readme
    */
    // devtools(),
    solidPlugin(),
  ],
  server: {
    port: 3000,
  },
  build: {
    target: 'esnext',
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-solid': ['solid-js', '@solidjs/router'],
          'vendor-query': ['@tanstack/solid-query'],
          'vendor-ui': ['@kobalte/core'],
          'vendor-charts': ['apexcharts', 'solid-apexcharts'],
          'vendor-viz': ['vis-network'],
        },
      },
    },
  },
});
