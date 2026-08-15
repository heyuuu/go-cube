import path from 'node:path';

import babel from '@rolldown/plugin-babel';
import tailwindcss from '@tailwindcss/vite';
import react, { reactCompilerPreset } from '@vitejs/plugin-react';
import { defineConfig, loadEnv } from 'vite';

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  // CUBE_API_TARGET：后端地址，默认 8080；air dev 起的 server 在 6001，用法：
  //   CUBE_API_TARGET=http://localhost:6001 pnpm dev
  const env = loadEnv(mode, process.cwd(), '');
  const apiTarget = env.CUBE_API_TARGET ?? 'http://localhost:8080';

  return {
    plugins: [react(), babel({ presets: [reactCompilerPreset()] }), tailwindcss()],
    resolve: {
      alias: {
        '@': path.resolve(import.meta.dirname, './src'),
      },
    },
    server: {
      proxy: {
        '/api': apiTarget,
        '/docs': apiTarget,
        '/openapi.json': apiTarget,
      },
    },
  };
});
