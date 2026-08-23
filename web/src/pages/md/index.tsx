import { ChevronDown, ChevronRight, Ellipsis, Folder, FileText } from 'lucide-react';
import { useCallback, useEffect, useRef, useState, type MouseEvent as ReactMouseEvent } from 'react';
import Markdown from 'react-markdown';
import { useSearchParams } from 'react-router';
import remarkGfm from 'remark-gfm';

import type { Opener } from '@/api/client';
import { ErrorBanner } from '@/components/error-banner';
import { TreeToolbar } from '@/components/tree-toolbar';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { buildFileTree, flattenFileTree, type FileTreeRow } from '@/lib/tree';
import { cn } from '@/lib/utils';
import { useMdContent, useMdList } from '@/queries/md';
import { useOpenerList, useOpenerOpen } from '@/queries/project';

// md 渲染页：独立于主应用 Layout（文档查看器，不带业务侧栏）。
// 路由 /md?path=<abs>，由 `cube md` 命令打开；渲染全在前端（后端只给原文与文件列表）。
// path 为目录时进双栏模式：左侧 md 文件树（无 md 的目录不出现），点文件显示内容、
// 点目录显示其 README.md（若有）；默认打开 = 点击根目录。path 为文件时单栏无侧栏。

// md 渲染主题：prose 默认 / GitHub（Primer 配色）。新主题 = index.css 加一组
// --tw-prose-* 变量 + 此处加一个条目，切换 UI 自动带上。
const MD_THEME_KEY = 'md.theme';
// 调色板取自各家官方规范（Primer / Atom / Dracula / Nord）；
// dark 主题会给页面壳挂 dark 类，整站 token 自动翻色（侧栏/正文背景一起暗）
const mdThemes = [
  { id: 'default', label: '默认', dark: false },
  { id: 'github', label: 'GitHub', dark: false },
  { id: 'solarized-light', label: 'Solarized Light', dark: false },
  { id: 'gruvbox-light', label: 'Gruvbox Light', dark: false },
  { id: 'nord-light', label: 'Nord Light', dark: false },
  { id: 'default-dark', label: '暗色', dark: true },
  { id: 'github-dark', label: 'GitHub Dark', dark: true },
  { id: 'one-dark', label: 'One Dark', dark: true },
  { id: 'dracula', label: 'Dracula', dark: true },
  { id: 'nord', label: 'Nord', dark: true },
] as const;
type MdThemeId = (typeof mdThemes)[number]['id'];

function loadMdTheme(): MdThemeId {
  const v = localStorage.getItem(MD_THEME_KEY);
  return mdThemes.some((t) => t.id === v) ? (v as MdThemeId) : 'default';
}

// 正文视图模式：渲染 / 源码 / 分栏（左渲染右源码对照）。持久化同主题。
const MD_VIEW_KEY = 'md.viewMode';
const mdViews = [
  { id: 'render', label: '渲染' },
  { id: 'source', label: '源码' },
  { id: 'split', label: '分栏' },
] as const;
type MdViewId = (typeof mdViews)[number]['id'];

function loadMdView(): MdViewId {
  const v = localStorage.getItem(MD_VIEW_KEY);
  return mdViews.some((t) => t.id === v) ? (v as MdViewId) : 'render';
}

// 侧栏宽度持久化：localStorage 记住用户拖出来的宽度，刷新不变
const SIDEBAR_WIDTH_KEY = 'md.sidebarWidth';
const SIDEBAR_WIDTH_DEFAULT = 256;
const SIDEBAR_WIDTH_MIN = 180;
const SIDEBAR_WIDTH_MAX = 640;

function loadSidebarWidth(): number {
  const v = Number(localStorage.getItem(SIDEBAR_WIDTH_KEY));
  return Number.isFinite(v) && v >= SIDEBAR_WIDTH_MIN && v <= SIDEBAR_WIDTH_MAX ? v : SIDEBAR_WIDTH_DEFAULT;
}

// resolveRel 把文内相对链接解析为绝对路径（基于当前文件所在目录，处理 ./ ../）
function resolveRel(currentFile: string, href: string): string {
  // 有的 md 源码里链接本身写成百分号编码（如 %E7%8E%B0），先解码再参与路径解析
  let rel = href.split(/[?#]/)[0];
  try {
    rel = decodeURIComponent(rel);
  } catch {
    // 坏编码保持原样，交给后续「未识别 → 新 Tab」分支兜底
  }
  const dir = currentFile.slice(0, currentFile.lastIndexOf('/'));
  const out: string[] = [];
  for (const p of `${dir}/${rel}`.split('/')) {
    if (p === '' || p === '.') continue;
    if (p === '..') out.pop();
    else out.push(p);
  }
  return `/${out.join('/')}`;
}

// MdLink 文内链接：外链开新页；.md 文件与目录链接站内导航（保持在主题化的查看器内，
// 避免整页跳出后落到亮色 404 页——此前「跳链接后暗色丢失」即此因）
function MdLink({
  href,
  base,
  onNavigate,
  children,
}: {
  href?: string;
  base: string;
  onNavigate: (absPath: string) => void;
  children: React.ReactNode;
}) {
  if (!href || /^[a-z]+:\/\//i.test(href) || href.startsWith('#')) {
    const external = !!href && /^[a-z]+:\/\//i.test(href);
    return external ? (
      <a href={href} target="_blank" rel="noreferrer">
        {children}
      </a>
    ) : (
      <a href={href ?? '#'}>{children}</a>
    );
  }
  return (
    <a
      href="#"
      onClick={(e) => {
        e.preventDefault();
        onNavigate(resolveRel(base, href));
      }}
    >
      {children}
    </a>
  );
}

// readmeOf 找 dir 目录下的 README.md（文件名不区分大小写）
function readmeOf(files: string[], dir: string): string | null {
  const prefix = dir.endsWith('/') ? dir : `${dir}/`;
  return files.find((f) => f.startsWith(prefix) && f.slice(prefix.length).toLowerCase() === 'readme.md') ?? null;
}

function MdTreeRow({
  row,
  selected,
  onFile,
  onDir,
  onExternal,
  openerList,
  open,
  onOpenNode,
  rowRef,
}: {
  row: FileTreeRow;
  selected: string | null;
  onFile: (path: string) => void;
  onDir: (path: string) => void;
  onExternal: (path: string) => void;
  openerList: Opener[];
  open: ReturnType<typeof useOpenerOpen>;
  onOpenNode: (path: string, app: string) => void;
  rowRef?: React.Ref<HTMLDivElement>;
}) {
  const n = row.node;
  const isDir = n.kind === 'dir';
  return (
    <div
      ref={rowRef}
      className={cn(
        'group flex cursor-pointer items-center gap-1 rounded-md py-1 pr-1 text-xs hover:bg-primary/15 dark:hover:bg-primary/25',
        isDir ? 'text-muted-foreground' : 'text-foreground',
        !isDir && n.path === selected && 'bg-primary/30 font-medium text-foreground dark:bg-primary/40',
      )}
      style={{ paddingLeft: row.depth * 16 + 4 }}
      onClick={() => (isDir ? onDir(n.path) : onFile(n.path))}
    >
      {isDir && row.hasChildren ? (
        <ChevronRight className={cn('size-3.5 shrink-0 transition-transform', row.expanded && 'rotate-90')} />
      ) : (
        <span className="w-3.5 shrink-0" />
      )}
      {isDir ? <Folder className="size-3.5 shrink-0" /> : <FileText className="size-3.5 shrink-0" />}
      <span className="min-w-0 flex-1 truncate" title={n.path}>
        {row.isRoot ? n.path : n.name}
      </span>
      {/* 节点菜单：目录与文件都有；opener 按路径类型过滤 role（目录 open-dir / 文件 open-file） */}
      <div
        className="shrink-0 opacity-0 transition-opacity group-hover:opacity-100"
        onClick={(e) => e.stopPropagation()}
      >
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="icon-sm" className="h-5 w-5 p-0" aria-label={`打开 ${n.name} 的方式`} />
            }
          >
            <Ellipsis className="size-3" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            <DropdownMenuItem onClick={() => onExternal(n.path)}>在新页面打开</DropdownMenuItem>
            <DropdownMenuSeparator />
            {openerList
              .filter((op) => (op.roles ?? []).includes(isDir ? 'open-dir' : 'open-file'))
              .map((op) => (
                <DropdownMenuItem key={op.name} disabled={open.isPending} onClick={() => onOpenNode(n.path, op.name)}>
                  {op.name}
                </DropdownMenuItem>
              ))}
            {openerList.filter((op) => (op.roles ?? []).includes(isDir ? 'open-dir' : 'open-file')).length === 0 && (
              <div className="px-2 py-1.5 text-xs text-muted-foreground">无可用 opener</div>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}

function MdContent({
  file,
  theme,
  onThemeChange,
  onNavigate,
  viewMode,
  onViewChange,
}: {
  file: string | null;
  theme: MdThemeId;
  onThemeChange: (t: MdThemeId) => void;
  onNavigate: (absPath: string) => void;
  viewMode: MdViewId;
  onViewChange: (v: MdViewId) => void;
}) {
  const themeLabel = mdThemes.find((t) => t.id === theme)?.label ?? theme;
  const q = useMdContent(file ?? '');

  // 分栏滚动同步：两边按滚动比例互相跟随（渲染段落与源码行无一一对应，
  // 只能做到近似对照——这是 diff 工具对非等高内容的通行做法）
  const renderRef = useRef<HTMLDivElement>(null);
  const sourceRef = useRef<HTMLDivElement>(null);
  const syncing = useRef(false);
  function syncScroll(from: 'render' | 'source') {
    if (syncing.current) return;
    const a = (from === 'render' ? renderRef : sourceRef).current;
    const b = (from === 'render' ? sourceRef : renderRef).current;
    if (!a || !b) return;
    const aMax = a.scrollHeight - a.clientHeight;
    const bMax = b.scrollHeight - b.clientHeight;
    if (aMax <= 0 || bMax <= 0) return;
    syncing.current = true;
    b.scrollTop = (a.scrollTop / aMax) * bMax;
    syncing.current = false;
  }

  if (!file) {
    return <div className="mt-10 text-center text-xs text-muted-foreground">此目录没有 README.md，从左侧选择文件</div>;
  }

  const fileName = file.split('/').pop() || '未命名';
  return (
    <>
      <header className="mb-6 flex items-start justify-between gap-4 border-b pb-4">
        <div className="min-w-0">
          <h1 className="text-lg font-semibold">{fileName}</h1>
          <div className="mt-1 font-mono text-xs text-muted-foreground" title={file}>
            {file}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-0.5 rounded-md border p-0.5">
          {mdViews.map((v) => (
            <button
              key={v.id}
              type="button"
              onClick={() => onViewChange(v.id)}
              className={cn(
                'rounded-sm px-2 py-0.5 text-xs transition-colors',
                viewMode === v.id
                  ? 'bg-background font-medium shadow-sm'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {v.label}
            </button>
          ))}
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger render={<Button variant="ghost" size="sm" className="h-6 shrink-0 px-2 text-xs" />}>
            主题：{themeLabel}
            <ChevronDown className="size-3" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {mdThemes
              .filter((t) => !t.dark)
              .map((t) => (
                <DropdownMenuItem key={t.id} onClick={() => onThemeChange(t.id)}>
                  {t.label}
                </DropdownMenuItem>
              ))}
            <DropdownMenuSeparator />
            {mdThemes
              .filter((t) => t.dark)
              .map((t) => (
                <DropdownMenuItem key={t.id} onClick={() => onThemeChange(t.id)}>
                  {t.label}
                </DropdownMenuItem>
              ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </header>
      {q.isPending && <div className="text-xs text-muted-foreground">加载中…</div>}
      {q.error && <ErrorBanner message={`读取失败：${q.error.message}`} />}
      {q.data && viewMode === 'render' && (
        <article
          className={cn(
            'prose prose-sm mb-10 max-w-none',
            theme === 'default' || theme === 'default-dark' ? 'dark:prose-invert' : `md-theme-${theme}`,
          )}
        >
          <Markdown
            remarkPlugins={[remarkGfm]}
            components={{
              a: ({ href, children }) => (
                <MdLink href={href} base={file} onNavigate={onNavigate}>
                  {children}
                </MdLink>
              ),
            }}
          >
            {q.data.content}
          </Markdown>
        </article>
      )}
      {q.data && viewMode === 'source' && (
        <pre className="mb-10 rounded-lg border bg-muted/30 p-4 font-mono text-xs break-words whitespace-pre-wrap">
          {q.data.content}
        </pre>
      )}
      {q.data && viewMode === 'split' && (
        <div className="grid gap-6 md:grid-cols-2">
          <div
            ref={renderRef}
            onScroll={() => syncScroll('render')}
            className="max-h-[calc(100dvh-11rem)] overflow-y-auto pr-1"
          >
            <article
              className={cn(
                'prose prose-sm max-w-none',
                theme === 'default' || theme === 'default-dark' ? 'dark:prose-invert' : `md-theme-${theme}`,
              )}
            >
              <Markdown
                remarkPlugins={[remarkGfm]}
                components={{
                  a: ({ href, children }) => (
                    <MdLink href={href} base={file} onNavigate={onNavigate}>
                      {children}
                    </MdLink>
                  ),
                }}
              >
                {q.data.content}
              </Markdown>
            </article>
          </div>
          <div
            ref={sourceRef}
            onScroll={() => syncScroll('source')}
            className="max-h-[calc(100dvh-11rem)] overflow-y-auto pr-1"
          >
            <pre className="rounded-lg border bg-muted/30 p-4 font-mono text-xs break-words whitespace-pre-wrap">
              {q.data.content}
            </pre>
          </div>
        </div>
      )}
    </>
  );
}

export function MdPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const path = searchParams.get('path') ?? '';
  const list = useMdList(path);
  const openers = useOpenerList();
  const open = useOpenerOpen();
  const [openError, setOpenError] = useState('');

  const dirMode = list.data?.dir === true;
  // 后端只给全量非忽略文件，.md 过滤归前端（单文件模式不做扩展名过滤，path 即所选文件）
  const allFiles = list.data?.files ?? [];
  const files = dirMode ? allFiles.filter((f) => f.toLowerCase().endsWith('.md')) : allFiles;
  const tree = dirMode ? buildFileTree(path, files) : null;

  // 选中文件走 URL 参数 file（天然按目录隔离、可分享、支持前进/后退）；
  // 缺省/失效时回落根 README（与「默认打开 = 点根目录」语义一致）
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set());
  const [sidebarW, setSidebarW] = useState(loadSidebarWidth);
  const [theme, setTheme] = useState(loadMdTheme);
  const [viewMode, setViewMode] = useState(loadMdView);

  function switchTheme(t: MdThemeId) {
    setTheme(t);
    localStorage.setItem(MD_THEME_KEY, t);
  }

  function switchView(v: MdViewId) {
    setViewMode(v);
    localStorage.setItem(MD_VIEW_KEY, v);
  }
  // file 参数支持相对路径（相对当前根，URL 更短）；绝对路径向后兼容
  const fileParamRaw = searchParams.get('file');
  const fileParam = !fileParamRaw
    ? null
    : fileParamRaw.startsWith('/')
      ? fileParamRaw
      : `${path}/${fileParamRaw.replace(/^\.?\//, '')}`;
  const selected = dirMode ? (files.includes(fileParam ?? '') ? fileParam : readmeOf(files, path)) : path || null;

  function selectFile(f: string) {
    setSearchParams({ path, file: f.startsWith(`${path}/`) ? f.slice(path.length + 1) : f });
  }

  const rows = tree ? flattenFileTree(tree, (p) => expanded.has(p)) : [];

  function expandAll() {
    if (!tree) return;
    const next = new Set<string>();
    const walk = (n: typeof tree) => {
      if (n.children.length > 0) {
        next.add(n.path);
        n.children.forEach(walk);
      }
    };
    walk(tree);
    setExpanded(next);
  }

  function collapseAll() {
    setExpanded(new Set());
  }

  // 拖拽右边缘调侧栏宽度；松手时把最终宽度写进 localStorage
  function startResize(e: ReactMouseEvent) {
    e.preventDefault();
    const startX = e.clientX;
    const startW = sidebarW;
    let w = startW;
    const onMove = (ev: MouseEvent) => {
      w = Math.min(SIDEBAR_WIDTH_MAX, Math.max(SIDEBAR_WIDTH_MIN, startW + ev.clientX - startX));
      setSidebarW(w);
    };
    const onUp = () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
      document.body.style.userSelect = '';
      localStorage.setItem(SIDEBAR_WIDTH_KEY, String(Math.round(w)));
    };
    document.body.style.userSelect = 'none'; // 拖拽中避免划选文字
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
  }

  function openNode(path: string, app: string) {
    setOpenError('');
    open.mutate({ path, app }, { onError: (e) => setOpenError(`打开失败：${e.message}`) });
  }

  // expandTo 展开目标路径的全部祖先目录节点（含自身；单链折叠节点的 path 是
  // 最深层目录，逐级前缀里必含它，多余的前缀键是无害的空操作）
  const expandTo = useCallback(
    (target: string) => {
      setExpanded((prev) => {
        const next = new Set(prev);
        const segs = target.slice(path.length).replace(/^\/+/, '').split('/');
        let cur = path;
        for (const seg of segs) {
          cur = `${cur}/${seg}`;
          next.add(cur);
        }
        return next;
      });
    },
    [path],
  );

  // 选中文件变化时（含刷新/直链进入）：展开其祖先目录并滚动定位到该行。
  // block:'nearest' 保证已在视野内时不跳动；用户手动折叠不受影响（再点击会重新展开）。
  const selectedRowRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!dirMode || !selected) return;
    // setState 放进 rAF 异步执行（React Compiler 禁止 effect 内同步 setState
    // 触发级联渲染）；双 rAF 等展开后的行渲染完成再滚动定位
    const raf = requestAnimationFrame(() => {
      expandTo(selected);
      requestAnimationFrame(() => selectedRowRef.current?.scrollIntoView({ block: 'nearest' }));
    });
    return () => cancelAnimationFrame(raf);
  }, [dirMode, selected, expandTo]);

  // 文内链接导航：根内 md 文件 → 选中并聚焦树节点；根内目录 → 展开 + 聚焦并
  // 显示其 README（与初始根语义一致）；根外或未识别目标 → 新 Tab 打开
  function navigateLink(absPath: string) {
    const inRoot = absPath === path || absPath.startsWith(`${path}/`);
    if (!inRoot) {
      window.open(`/md?path=${encodeURIComponent(absPath)}`, '_blank');
      return;
    }
    if (files.includes(absPath)) {
      selectFile(absPath);
      expandTo(absPath);
    } else if (files.some((f) => f.startsWith(`${absPath}/`))) {
      // 目录链接：展开聚焦；正文保持不动（点目录不切正文的既定语义）
      expandTo(absPath);
    } else {
      window.open(`/md?path=${encodeURIComponent(absPath)}`, '_blank');
    }
  }

  // 新页面（新标签）打开单文件模式
  function openExternal(file: string) {
    window.open(`/md?path=${encodeURIComponent(file)}`, '_blank');
  }

  function onDir(dirPath: string) {
    // 点目录只做折叠/展开，不切正文——折叠时误切当前文档并非本意（初始根 README 见上方 selected 推导）
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(dirPath)) next.delete(dirPath);
      else next.add(dirPath);
      return next;
    });
  }

  if (!path) {
    return (
      <main className="mx-auto max-w-3xl px-6 py-10">
        <ErrorBanner message="缺少 path 参数：请通过 cube md <file|dir> 打开本页" />
      </main>
    );
  }

  const listError = list.error ? `读取路径失败：${list.error.message}` : '';

  const themeDark = mdThemes.find((t) => t.id === theme)?.dark === true;

  return (
    <div className={cn('flex h-dvh bg-background text-foreground', themeDark && 'dark')}>
      {dirMode && (
        <>
          <aside className="shrink-0 overflow-y-auto border-r p-3" style={{ width: sidebarW - 4 }}>
            {/* 共用目录树工具条（与工作台代码阅读面板同款）；额外按钮经 extra 注入 */}
            <TreeToolbar onExpandAll={expandAll} onCollapseAll={collapseAll} />
            {list.isPending && <div className="text-xs text-muted-foreground">加载中…</div>}
            {listError && <div className="text-xs text-muted-foreground">{listError}</div>}
            {!list.isPending && !listError && rows.length <= 1 && (
              <div className="text-xs text-muted-foreground">未找到 markdown 文件</div>
            )}
            {rows.map((row) => (
              <MdTreeRow
                key={row.node.path}
                row={row}
                selected={selected}
                onFile={selectFile}
                onDir={onDir}
                onExternal={openExternal}
                openerList={openers.data?.list ?? []}
                open={open}
                onOpenNode={openNode}
              />
            ))}
          </aside>
          {/* 拖拽调宽的把手：独立成列（不放进 aside），侧栏滚动条再长也不挡它 */}
          <div onMouseDown={startResize} className="w-1 shrink-0 cursor-col-resize hover:bg-primary/40" aria-hidden />
        </>
      )}

      <main className="min-w-0 flex-1 overflow-y-auto px-6 py-8">
        <div className={cn(viewMode === 'split' ? 'w-full' : 'mx-auto max-w-3xl')}>
          {openError && <ErrorBanner message={openError} />}
          {list.isPending && <div className="text-xs text-muted-foreground">加载中…</div>}
          {listError && <ErrorBanner message={listError} />}
          {!list.isPending && !listError && (
            <MdContent
              file={selected}
              theme={theme}
              onThemeChange={switchTheme}
              onNavigate={navigateLink}
              viewMode={viewMode}
              onViewChange={switchView}
            />
          )}
        </div>
      </main>
    </div>
  );
}
