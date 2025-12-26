import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'
import fs from 'fs'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    https: {
      key: fs.readFileSync(path.resolve(__dirname, '../certs/server.key')),
      cert: fs.readFileSync(path.resolve(__dirname, '../certs/server.crt')),
    },
    port: 3001,
    proxy: {
      '/api': {
        target: 'https://localhost:8081',
        changeOrigin: true,
        secure: false,
      },
      '/ws': {
        target: 'wss://localhost:8081',
        ws: true,
        secure: false,
      }
    }
  }
})
