import { css } from '@codemirror/lang-css';
// CodeMirror 语言映射：按扩展名选择（提案 1012）。未知类型纯文本。
import { go } from '@codemirror/lang-go';
import { html } from '@codemirror/lang-html';
import { java } from '@codemirror/lang-java';
import { javascript } from '@codemirror/lang-javascript';
import { json } from '@codemirror/lang-json';
import { markdown } from '@codemirror/lang-markdown';
import { python } from '@codemirror/lang-python';
import { rust } from '@codemirror/lang-rust';
import { sql } from '@codemirror/lang-sql';
import type { Extension } from '@codemirror/state';

const langMap: Record<string, () => Extension> = {
  go: () => go(),
  json: () => json(),
  jsonc: () => json(),
  md: () => markdown(),
  markdown: () => markdown(),
  py: () => python(),
  js: () => javascript(),
  mjs: () => javascript(),
  cjs: () => javascript(),
  jsx: () => javascript({ jsx: true }),
  ts: () => javascript({ typescript: true }),
  tsx: () => javascript({ typescript: true, jsx: true }),
  css: () => css(),
  scss: () => css(),
  html: () => html(),
  htm: () => html(),
  xml: () => html(),
  vue: () => html(),
  rs: () => rust(),
  java: () => java(),
  sql: () => sql(),
};

export function languageForFile(file: string): Extension {
  const ext = file.split('.').pop()?.toLowerCase() ?? '';
  return langMap[ext] ? langMap[ext]() : [];
}
