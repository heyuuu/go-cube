import Markdown from 'react-markdown';
import remarkFrontmatter from 'remark-frontmatter';
import remarkGfm from 'remark-gfm';

import { parseFrontmatter, type FrontMatterEntry } from '@/lib/frontmatter';
import { cn } from '@/lib/utils';

// markdown 渲染公共件：md 独立页（/md，带主题）与工作台内容面板预览共用同一管线
// （react-markdown + gfm + frontmatter 剥离）。主题差异经 className 注入，
// 链接等组件覆盖经 components 透传（md 页的站内导航链接即由此接入）。

// 渲染主题：每个主题自带亮色/暗色两套变量（index.css 的 .md-theme-* + .dark .md-theme-*），
// 跟随整站亮暗切换，无需换主题。新主题 = index.css 加亮/暗两组 --tw-prose-* 变量
// + 此处加一个条目。
export const mdThemes = [
  { id: 'default', label: '默认' },
  { id: 'github', label: 'GitHub' },
  { id: 'solarized', label: 'Solarized' },
  { id: 'gruvbox', label: 'Gruvbox' },
  { id: 'nord', label: 'Nord' },
] as const;
export type MdThemeId = (typeof mdThemes)[number]['id'];

// 旧版（亮暗分开两套主题）id → 合并后主题的迁移
const legacyMdThemes: Record<string, MdThemeId> = {
  'default-dark': 'default',
  'github-dark': 'github',
  'solarized-light': 'solarized',
  'gruvbox-light': 'gruvbox',
  'nord-light': 'nord',
  // 无亮色对应、被合并淘汰的主题回落默认
  'one-dark': 'default',
  dracula: 'default',
};

// 主题 class：默认主题走 prose 暗色反转，其余主题整块换配色（md-theme-*）
export function mdThemeCls(theme: MdThemeId): string {
  return theme === 'default' ? 'dark:prose-invert' : `md-theme-${theme}`;
}

export function loadMdTheme(key: string): MdThemeId {
  const v = localStorage.getItem(key) ?? '';
  if (mdThemes.some((t) => t.id === v)) return v as MdThemeId;
  return legacyMdThemes[v] ?? 'default';
}

// front-matter 元信息区：文档头 --- 块解析出的扁平字段（title/date/tags 等）以键值
// 展示在正文前；源码视图保持原文（front-matter 本就是源码的一部分）
export function FrontMatterBlock({ entries }: { entries: FrontMatterEntry[] }) {
  return (
    <div className="mb-6 rounded-lg border bg-muted/30 px-4 py-2 font-mono text-xs">
      {entries.map((e) => (
        <div key={e.key} className="flex gap-3 py-1">
          <span className="w-24 shrink-0 text-muted-foreground">{e.key}</span>
          <span className="min-w-0 break-words whitespace-pre-line">{e.value}</span>
        </div>
      ))}
    </div>
  );
}

export function MarkdownView({
  content,
  className,
  components,
}: {
  content: string;
  className?: string; // prose 主题类（md 页的 md-theme-* / dark:prose-invert）
  components?: Parameters<typeof Markdown>[0]['components'];
}) {
  const fm = parseFrontmatter(content);
  return (
    <>
      {fm.length > 0 && <FrontMatterBlock entries={fm} />}
      <article className={cn('prose prose-sm max-w-none', className)}>
        <Markdown remarkPlugins={[remarkGfm, remarkFrontmatter]} components={components}>
          {content}
        </Markdown>
      </article>
    </>
  );
}
