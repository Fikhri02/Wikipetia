/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: {
        name: 'WikiPetia',
        short_name: 'WikiPetia',
        description: 'Learn a course, prove you know it.',
        start_url: '/',
        display: 'standalone',
        background_color: '#ffffff',
        theme_color: '#0f172a',
      },
    }),
  ],
  server: {
    // A dedicated port: 5173 is Vite's default and is frequently already taken
    // by another project on this machine. strictPort makes a clash fail loudly
    // instead of silently drifting to another port and breaking the e2e baseURL.
    port: 5180,
    strictPort: true,
    // The API runs on 8080; proxying keeps the browser on one origin, so no
    // CORS configuration is needed on the Go side.
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true, rewrite: (p) => p.replace(/^\/api/, '') },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
    css: false,
  },
})
