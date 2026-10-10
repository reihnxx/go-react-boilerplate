/// <reference types="vitest/config" />
import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

// The SPA and the API share one origin: in development Vite proxies /api to
// the Go server, in production a reverse proxy does the same.
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, import.meta.dirname, '')
  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: { '@': path.resolve(import.meta.dirname, './src') },
    },
    build: {
      rolldownOptions: {
        output: {
          // Stable vendor chunks cache well across app releases.
          codeSplitting: {
            groups: [
              { name: 'vendor-react', test: /node_modules[\\/](react|react-dom|scheduler|react-router)[\\/]/ },
              { name: 'vendor-ui', test: /node_modules[\\/](radix-ui|@radix-ui|@floating-ui|sonner|lucide-react)[\\/]/ },
              { name: 'vendor-data', test: /node_modules[\\/](@tanstack|react-hook-form|@hookform|zod)[\\/]/ },
            ],
          },
        },
      },
    },
    server: {
      port: 5173,
      proxy: {
        '/api': { target: env.VITE_API_PROXY_TARGET || 'http://127.0.0.1:8080', changeOrigin: false },
      },
    },
    test: {
      environment: 'jsdom',
      globals: false,
      setupFiles: ['./src/test/setup.ts'],
      css: false,
      restoreMocks: true,
      coverage: {
        provider: 'v8',
        include: ['src/**/*.{ts,tsx}'],
        // Generated shadcn components and test helpers are not our code under test.
        exclude: ['src/components/ui/**', 'src/test/**', 'src/**/*.test.{ts,tsx}', 'src/main.tsx', 'src/types/**'],
      },
    },
  }
})
