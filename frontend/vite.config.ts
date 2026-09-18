import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'

// https://vitejs.dev/config/
export default defineConfig({
  base: '/',
  plugins: [vue()],
  server: {
    proxy: { '/api': { target: 'http://127.0.0.1:9090', ws: true } },
    watch: {
      ignored: ['**/node_modules/**', '**/.git/**', '**/dist/**'],
    },
  },
  resolve: {
    extensions: ['.ts', '.js'],
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      vue: 'vue/dist/vue.esm-bundler.js',
    },
  },
  build: {
    cssCodeSplit: false,
    chunkSizeWarningLimit: 4096, // 4MB
    rolldownOptions: {
      output: {
        strictExecutionOrder: true,
        codeSplitting: {
          groups: [
            { name: 'vue', test: /node_modules\/vue/ },
            { name: 'codemirror', test: /node_modules\/@codemirror/ },
            { name: 'prettier', test: /node_modules\/prettier/ },
            { name: 'vendor', test: /node_modules/ },
            { name: 'index' },
          ],
        },
      },
    },
  },
})
