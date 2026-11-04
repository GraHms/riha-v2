import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// In development the Go API runs on :8080; Vite proxies API calls to it.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/v1': 'http://localhost:8080',
    },
  },
})
