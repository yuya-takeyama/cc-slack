import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  base: '/web/',
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true
  }
})