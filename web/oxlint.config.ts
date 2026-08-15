import { defineConfig } from 'oxlint';

export default defineConfig({
  plugins: ['react', 'typescript', 'oxc'],
  rules: {
    'react/react-compiler': 'error',
    'react/rules-of-hooks': 'error',
    'react/only-export-components': ['warn', { allowConstantExport: true }],
  },
  overrides: [
    {
      files: ['src/components/ui/**'],
      rules: {
        // shadcn CLI 的 vendored 组件：cva variants 与组件同文件导出是官方模式，
        // 未来每次 shadcn add 都会带来新的，不为它改生成代码，仅此目录关闭
        'react/only-export-components': 'off',
      },
    },
  ],
});
