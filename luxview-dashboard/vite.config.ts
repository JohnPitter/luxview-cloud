import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const CHUNK_SIZE_WARNING_LIMIT_KB = 1600;

export default defineConfig({
  plugins: [react()],
  build: {
    chunkSizeWarningLimit: CHUNK_SIZE_WARNING_LIMIT_KB,
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
});
