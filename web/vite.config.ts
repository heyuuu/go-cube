import path from 'node:path';

import babel from '@rolldown/plugin-babel';
import tailwindcss from '@tailwindcss/vite';
import react, { reactCompilerPreset } from '@vitejs/plugin-react';
import { defineConfig, loadEnv, type Plugin } from 'vite';

// 把浏览器端的 console.error / 未捕获异常转发到 dev server 终端。
// vite 只负责转译和 HMR，浏览器里的报错终端天然看不到，排查时得开 devtools；
// 这里在页面注入一小段脚本，通过 /__dev/console 回传并打印到 stdout。
function browserConsoleToTerminal(): Plugin {
  const endpoint = '/__dev/console';
  return {
    name: 'browser-console-to-terminal',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use(endpoint, (req, res) => {
        let body = '';
        req.on('data', (chunk) => (body += chunk));
        req.on('end', () => {
          try {
            const { level, args } = JSON.parse(body) as { level: string; args: unknown[] };
            const line = args.map((a) => (typeof a === 'string' ? a : JSON.stringify(a))).join(' ');
            const tag = level === 'error' ? '\x1b[31m[browser error]\x1b[0m' : '[browser warn]';
            console.warn(`${tag} ${line}`);
          } catch {
            // 转发失败不影响页面
          }
          res.statusCode = 204;
          res.end();
        });
      });
    },
    transformIndexHtml() {
      return [
        {
          tag: 'script',
          attrs: { type: 'module' },
          children: `
const send = (level, args) => {
  try { fetch('${endpoint}', { method: 'POST', body: JSON.stringify({ level, args }) }); } catch {}
};
const fmt = (a) => a instanceof Error ? (a.stack ?? a.message) : a;
const origError = console.error.bind(console);
console.error = (...args) => { send('error', args.map(fmt)); origError(...args); };
const origWarn = console.warn.bind(console);
console.warn = (...args) => { send('warn', args.map(fmt)); origWarn(...args); };
window.addEventListener('error', (e) => send('error', [e.error ?? e.message]));
window.addEventListener('unhandledrejection', (e) => send('error', ['Unhandled rejection:', e.reason]));
`,
        },
      ];
    },
  };
}

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  // CUBE_API_TARGET：后端地址，默认 8080；air dev 起的 server 在 6001，用法：
  //   CUBE_API_TARGET=http://localhost:6001 pnpm dev
  const env = loadEnv(mode, process.cwd(), '');
  const apiTarget = env.CUBE_API_TARGET ?? 'http://localhost:6001';

  return {
    plugins: [react(), babel({ presets: [reactCompilerPreset()] }), tailwindcss(), browserConsoleToTerminal()],
    resolve: {
      alias: {
        '@': path.resolve(import.meta.dirname, './src'),
      },
    },
    server: {
      proxy: {
        // ws: true —— pty 终端走 WebSocket upgrade（/api/workbench/pty），
        // 字符串简写不代理 upgrade 请求，终端面板会一直「连接中…」后失败
        '/api': { target: apiTarget, ws: true },
        '/docs': apiTarget,
        '/openapi.json': apiTarget,
      },
    },
  };
});
