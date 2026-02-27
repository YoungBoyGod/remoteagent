import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig(({ mode }) => {
  const rootDir = resolve(__dirname, '..')
  const env = loadEnv(mode, rootDir, '')
  const frontendPort = Number(env.FRONTEND_PORT || '7000')
  const frontendHost = env.FRONTEND_HOST || '0.0.0.0'
  const proxyTarget = env.FRONTEND_PROXY_TARGET || 'http://localhost:40001'

  return {
    envDir: rootDir,
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
      host: frontendHost,
      port: frontendPort,
      proxy: {
        '/api': {
          target: proxyTarget,
          changeOrigin: true,
        },
        '/healthz': {
          target: proxyTarget,
          changeOrigin: true,
        },
        '/metrics': {
          target: proxyTarget,
          changeOrigin: true,
        },
      },
    },
  }
})
