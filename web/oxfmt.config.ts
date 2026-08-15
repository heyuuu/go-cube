import { defineConfig } from 'oxfmt';

export default defineConfig({
  printWidth: 120,
  singleQuote: true,
  // sort imports
  sortImports: {
    groups: [
      // 内置或第三方包
      'builtin',
      'external',
      'internal',
      // 项目包
      'parent',
      'index',
      'sibling',
      'unknown',
      // 副作用包
      'side_effect',
      'side_effect_style',
    ],
  },
});
