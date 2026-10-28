import { fileURLToPath, URL } from 'node:url'

import vue from '@vitejs/plugin-vue'
import { loadEnv } from 'vite'
import { defineConfig } from 'vitest/config'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  return {
    plugins: [vue()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      port: 5174,
      proxy: {
        '/cp': {
          target: env.VITE_CP_PROXY_TARGET || 'http://localhost:8080',
          changeOrigin: true,
        },
      },
    },
    test: {
      environment: 'jsdom',
      include: ['src/**/*.test.ts'],
    },
  }
})
