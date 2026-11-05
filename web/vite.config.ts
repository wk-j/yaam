import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/',
  build: {
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
    // One bundle served from loopback and embedded in the binary; CodeMirror
    // alone is most of it, so splitting buys nothing here.
    chunkSizeWarningLimit: 1024,
  },
})
