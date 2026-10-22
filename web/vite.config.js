import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Dev proxy: Vite serves the SPA and forwards API calls to the Go server.
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/textures': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
  build: { outDir: 'dist', emptyOutDir: true },
})
