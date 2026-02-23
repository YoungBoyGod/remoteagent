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
  optimizeDeps: {
    include: [
      '@tiptap/vue-3',
      '@tiptap/starter-kit',
      '@tiptap/extension-image',
      '@tiptap/extension-placeholder',
      '@tiptap/extension-underline',
      '@tiptap/extension-text-align',
      '@tiptap/extension-highlight',
      '@tiptap/extension-task-list',
      '@tiptap/extension-task-item',
      '@tiptap/extension-table',
      '@tiptap/extension-table-row',
      '@tiptap/extension-table-cell',
      '@tiptap/extension-table-header',
      '@tiptap/extension-code-block-lowlight',
      'tiptap-markdown',
      'lowlight',
    ],
  },
  build: {
    chunkSizeWarningLimit: 5000,
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
