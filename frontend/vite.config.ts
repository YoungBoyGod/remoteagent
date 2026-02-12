import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 7000,
    proxy: {
      '/api': {
        target: 'http://localhost:40001',
        changeOrigin: true,
      },
      '/healthz': {
        target: 'http://localhost:40001',
        changeOrigin: true,
      },
      '/metrics': {
        target: 'http://localhost:40001',
        changeOrigin: true,
      },
    },
  },
})
