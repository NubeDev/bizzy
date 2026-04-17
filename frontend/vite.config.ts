import path from "path"
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8090',
        ws: true,
      },
      '/health': 'http://localhost:8090',
      '/users/me': 'http://localhost:8090',
      '/my/tools': 'http://localhost:8090',
      '/my/prompts': 'http://localhost:8090',
    },
  },
})
