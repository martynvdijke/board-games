import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  root: 'web',
  base: '/static/',
  plugins: [tailwindcss()],
  build: {
    outDir: fileURLToPath(new URL('./internal/web/static', import.meta.url)),
    emptyOutDir: true,
    manifest: true,
    assetsDir: 'assets',
    rollupOptions: {
      input: fileURLToPath(new URL('./web/src/main.ts', import.meta.url)),
    },
  },
})
