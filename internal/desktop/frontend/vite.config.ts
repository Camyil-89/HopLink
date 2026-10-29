import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
export default defineConfig({
  plugins: [react()],
  build: { outDir: '../app/assets/ui/dist', emptyOutDir: true },
  server: { port: 5173, strictPort: true },
})
