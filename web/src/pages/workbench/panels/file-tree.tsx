import { ChevronDown, ChevronRight, Crosshair, FileText, Folder } from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { TreeToolbar } from '@/components/tree-toolbar';
import { Button } from '@/components/ui/button';
import { relativeFilePath } from '@/lib/path';
import { buildFileTree, computeDirStatuses, flattenFileTree, type FileTreeNode } from '@/lib/tree';
import { cn } from '@/lib/utils';
import { useWorkbenchTree } from '@/queries/workbench';

import type { TreeSource } from '../params';

// 文件树：一次全量拉取 git 管理的文件清单（扁平相对路径），前端用 lib/tree 组树——
// 与 md 页同一套组树/拍平/折叠链逻辑。数据全在内存，展开/折叠/全部展开均为本地状态切换。
// 受控组件（供代码阅读/diff 面板复用）：selectedFile/onPick 由调用方管理。
// filter 生效时只显示集合内文件（及其祖先目录）——差异模式用。
// viewMode：树形（目录可展开）/ 平摊（每行一个文件，显示相对根目录的完整路径），
// scope（全量/差异文件）与模式切换按钮渲染在树工具条（extra），状态由调用方持有。
export type FileStat = {
  adds: number;
  dels: number;
  binary?: boolean;
  status: string; // added / modified / deleted / renamed（后端 DiffEntry）
  oldPath?: string; // rename 的旧路径
};

// 状态配色：新增绿（与 +N 同色系）/ 修改琥珀 / 删除红（与 -N 同色系）/ 重命名紫
const STATUS_COLOR: Record<string, { icon: string; text: string }> = {
  added: { icon: 'text-emerald-500', text: 'text-emerald-600 dark:text-emerald-400' },
  modified: { icon: 'text-amber-500', text: 'text-amber-600 dark:text-amber-400' },
  deleted: { icon: 'text-red-500', text: 'text-red-600 dark:text-red-400' },
  renamed: { icon: 'text-violet-500', text: 'text-violet-600 dark:text-violet-400' },
};

// 展开偏好：只记「上一次点的全部展开/全部折叠」，不逐目录记——
// 刷新后整棵树按上次的 bulk 动作恢复（个人偏好，localStorage）
const TREE_EXPAND_KEY = 'cube.workbench.codetree.expand';

function allDirPaths(root: FileTreeNode): Set<string> {
  const next = new Set<string>();
  const walk = (n: FileTreeNode) => {
    if (n.children.length > 0) {
      next.add(n.path);
      n.children.forEach(walk);
    }
  };
  root.children.forEach(walk);
  return next;
}

export function FileTree({
  path,
  source,
  selectedFile,
  onPick,
  filter,
  stats,
  statsPending,
  viewMode,
  onViewMode,
  scope,
  onScope,
  scopePending,
}: {
  path: string;
  source: TreeSource;
  selectedFile: string;
  onPick: (file: string) => void;
  filter: Set<string> | null;
  stats: Map<string, FileStat> | null; // 差异文件的行级统计（差异模式），行尾显示 +N -N
  statsPending: boolean; // 统计/清单未就绪时不渲染行——避免先闪全量再过滤/着色
  viewMode: 'tree' | 'flat';
  onViewMode: (m: 'tree' | 'flat') => void;
  scope: 'all' | 'diff';
  onScope: (s: 'all' | 'diff') => void;
  scopePending: boolean;
}) {
  // 展开状态提升到树级统一管理（按目录相对路径），行组件无状态渲染
  const [expandedSet, setExpandedSet] = useState<ReadonlySet<string>>(() => new Set(['']));
  const tree = useWorkbenchTree(path, source);

  // 组树输入按模式分叉：全量 = ls-files 清单（worktree 源并入已删除路径——
  // 它们不在 ls-files 里，不并入就不会出现在树中；commit/ref 源不并，
  // 树语义是该提交的内容）；差异 = 只对变更文件集组树——不能用全量树过滤行，
  // 否则过滤后才变单链的目录（父目录的其他子项被滤掉）吃不到压缩逻辑
  const root = useMemo(() => {
    if (filter) return buildFileTree('', [...filter]);
    const list = tree.data?.list ?? [];
    if (!list.length) return null;
    let files = list as string[];
    if (stats && source.type === 'worktree') {
      const inList = new Set(list);
      const deleted = [...stats.entries()].filter(([p, s]) => s.status === 'deleted' && !inList.has(p)).map(([p]) => p);
      if (deleted.length) files = [...list, ...deleted];
    }
    return buildFileTree('', files);
  }, [tree.data, stats, source.type, filter]);

  // 目录行配色：状态由全量清单 + 变更集推导（基准版文件集可从二者反推，见 lib/tree）。
  // 用原始 list（不含上面并入的删除路径）——「现在有子文件」以真实现存文件为准
  const dirStatuses = useMemo(
    () => (stats ? computeDirStatuses(stats, tree.data?.list ?? []) : null),
    [stats, tree.data],
  );

  const toggle = useCallback((dir: string) => {
    setExpandedSet((prev) => {
      const next = new Set(prev);
      if (next.has(dir)) {
        next.delete(dir);
      } else {
        next.add(dir);
      }
      return next;
    });
  }, []);

  const expandAll = useCallback(() => {
    if (!root) return;
    setExpandedSet(allDirPaths(root));
    localStorage.setItem(TREE_EXPAND_KEY, 'all');
  }, [root]);

  const collapseAll = useCallback(() => {
    setExpandedSet(new Set(['']));
    localStorage.setItem(TREE_EXPAND_KEY, 'none');
  }, []);

  // 首次拿到树数据时按上次的 bulk 动作恢复（仅一次；之后的手动展开/折叠不记忆）
  const bulkInitRef = useRef(false);
  useEffect(() => {
    if (bulkInitRef.current || !root) return;
    bulkInitRef.current = true;
    const saved = localStorage.getItem(TREE_EXPAND_KEY);
    if (saved === 'all') setExpandedSet(allDirPaths(root));
    else if (saved === 'none') setExpandedSet(new Set(['']));
  }, [root]);

  const listRef = useRef<HTMLDivElement>(null);
  // 定位当前文件：展开其祖先目录 → 滚动到该行 → 闪烁高亮（class 命令式添加，同 git 树定位）
  const locateCurrent = useCallback(() => {
    if (!selectedFile) return;
    setExpandedSet((prev) => {
      const next = new Set(prev);
      const parts = selectedFile.split('/');
      for (let i = 1; i < parts.length; i++) next.add(parts.slice(0, i).join('/'));
      return next;
    });
    // rAF 等展开后的 DOM 提交再滚动定位
    requestAnimationFrame(() => {
      const el = listRef.current?.querySelector(`[data-path="${selectedFile}"]`);
      if (!el) return;
      el.scrollIntoView({ block: 'center' });
      el.classList.remove('row-flash');
      void (el as HTMLElement).offsetWidth; // 重启动画
      el.classList.add('row-flash');
      window.setTimeout(() => el.classList.remove('row-flash'), 2500);
    });
  }, [selectedFile]);

  // 可见行：树形 = flatten 后按差异过滤（文件须在集合内，目录须有集合内文件位于其下）；
  // 平摊 = 直接用接口的扁平路径清单（排序对齐树形的字典序），无目录行。
  // 根行（isRoot）不渲染，面板标题已提供上下文
  const rows = useMemo(() => {
    if (!root) return [];
    if (viewMode === 'flat') {
      return flattenFileTree(root, () => true)
        .filter((r) => r.node.kind === 'file' && (!filter || filter.has(r.node.path)))
        .map((r) => ({ node: r.node, depth: 0, expanded: false }));
    }
    return flattenFileTree(root, (p) => expandedSet.has(p)).filter((r) => {
      if (r.isRoot) return false;
      if (!filter) return true;
      return r.node.kind === 'dir' ? hasFileUnder(filter, r.node.path) : filter.has(r.node.path);
    });
  }, [root, expandedSet, filter, viewMode]);

  const flat = viewMode === 'flat';

  return (
    <div className="flex h-full flex-col">
      <TreeToolbar
        onExpandAll={expandAll}
        onCollapseAll={collapseAll}
        leading={
          <Button
            variant="ghost"
            size="icon-sm"
            className="text-muted-foreground"
            disabled={!selectedFile}
            onClick={locateCurrent}
            title="定位当前文件"
            aria-label="定位当前文件"
          >
            <Crosshair className="size-3.5" />
          </Button>
        }
        extra={
          <>
            <div className="ml-auto flex items-center gap-1.5">
              <div className="flex overflow-hidden rounded-md border border-border text-[10px]">
                {(
                  [
                    ['tree', '树形'],
                    ['flat', '平摊'],
                  ] as const
                ).map(([m, label]) => (
                  <button
                    key={m}
                    type="button"
                    className={cn(
                      'px-1.5 py-0.5 transition-colors',
                      viewMode === m
                        ? 'bg-primary/15 font-medium text-primary'
                        : 'text-muted-foreground hover:bg-accent',
                    )}
                    onClick={() => onViewMode(m)}
                    title={m === 'flat' ? '平摊列表（显示完整路径）' : '树形目录'}
                  >
                    {label}
                  </button>
                ))}
              </div>
              <div className="flex overflow-hidden rounded-md border border-border text-[10px]">
                {(
                  [
                    ['all', '全量'],
                    ['diff', '差异'],
                  ] as const
                ).map(([m, label]) => (
                  <button
                    key={m}
                    type="button"
                    disabled={scope === 'diff' && scopePending}
                    className={cn(
                      'px-1.5 py-0.5 transition-colors',
                      scope === m ? 'bg-primary/15 font-medium text-primary' : 'text-muted-foreground hover:bg-accent',
                    )}
                    onClick={() => onScope(m)}
                    title={m === 'diff' ? '只看相对上一版本的变更文件' : '查看全部文件'}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>
          </>
        }
      />
      <div ref={listRef} className="min-h-0 flex-1 overflow-y-auto p-1 text-xs">
        {tree.isPending || statsPending ? (
          <div className="py-1 pl-3 text-muted-foreground">加载中…</div>
        ) : tree.isError ? (
          <div className="py-1 pl-3 text-destructive">{tree.error.message}</div>
        ) : rows.length === 0 ? (
          <div className="py-1 pl-3 text-muted-foreground">（{filter ? '无变更文件' : '空目录'}）</div>
        ) : (
          rows.map(({ node, depth, expanded }) =>
            node.kind === 'dir' && !flat ? (
              (() => {
                const dColor = dirStatuses ? STATUS_COLOR[dirStatuses.get(node.path) ?? ''] : undefined;
                return (
                  <button
                    key={node.path}
                    type="button"
                    data-path={node.path}
                    className="flex w-full items-center gap-1 rounded px-1 py-0.5 text-left hover:bg-accent"
                    style={{ paddingLeft: depth * 12 + 4 }}
                    onClick={() => toggle(node.path)}
                  >
                    {expanded ? (
                      <ChevronDown className="size-3 shrink-0" />
                    ) : (
                      <ChevronRight className="size-3 shrink-0" />
                    )}
                    <Folder className={cn('size-3.5 shrink-0', dColor ? dColor.icon : 'text-muted-foreground')} />
                    <span className={cn('truncate', dColor && dColor.text)}>{node.name}</span>
                  </button>
                );
              })()
            ) : stats?.get(node.path)?.status === 'deleted' ? (
              // 删除文件：磁盘与 index 均无，不可选不可读；并入树后与其他差异行对齐
              <div
                key={node.path}
                data-path={node.path}
                title={node.path}
                className="flex w-full items-center gap-1 rounded px-1 py-0.5 text-left"
                style={{ paddingLeft: flat ? 6 : depth * 12 + 16 }}
              >
                <FileText className="size-3.5 shrink-0 text-red-500" />
                <span className="truncate text-red-600 line-through dark:text-red-400">
                  {flat ? node.path : node.name}
                </span>
                {statLine(stats.get(node.path))}
              </div>
            ) : (
              <button
                key={node.path}
                type="button"
                data-path={node.path}
                title={node.path}
                className={cn(
                  'flex w-full items-center gap-1 rounded px-1 py-0.5 text-left hover:bg-accent',
                  selectedFile === node.path && 'bg-primary/15 font-medium',
                )}
                style={{ paddingLeft: flat ? 6 : depth * 12 + 16 }}
                onClick={() => onPick(node.path)}
              >
                {(() => {
                  const stat = stats?.get(node.path);
                  const color = stat ? STATUS_COLOR[stat.status] : undefined;
                  // rename 行括号附注旧路径（相对新路径所在目录，如 ./a.md、../xx/a.md）：
                  // 只改目录不改名时，旧 basename 看不出旧文件在哪
                  const oldNote =
                    stat?.status === 'renamed' && stat.oldPath
                      ? `（${relativeFilePath(node.path, stat.oldPath)}）`
                      : '';
                  return (
                    <>
                      <FileText className={cn('size-3.5 shrink-0', color ? color.icon : 'text-muted-foreground')} />
                      <span className={cn('truncate', color && color.text)}>
                        {(flat ? node.path : node.name) + oldNote}
                      </span>
                      {statLine(stat)}
                    </>
                  );
                })()}
              </button>
            ),
          )
        )}
      </div>
    </div>
  );
}

function hasFileUnder(filter: Set<string>, dir: string): boolean {
  const prefix = dir + '/';
  for (const f of filter) {
    if (f.startsWith(prefix)) return true;
  }
  return false;
}

// 差异行的行数统计（+N 绿 / -N 红，右对齐）；二进制显示 bin
function statLine(stat?: FileStat) {
  if (!stat) return null;
  return (
    <span className="ml-auto shrink-0 font-mono text-[10px]">
      {stat.binary ? (
        <span className="text-muted-foreground">bin</span>
      ) : (
        <>
          {stat.adds > 0 ? <span className="text-emerald-600 dark:text-emerald-400">+{stat.adds}</span> : null}
          {stat.adds > 0 && stat.dels > 0 ? ' ' : null}
          {stat.dels > 0 ? <span className="text-red-600 dark:text-red-400">-{stat.dels}</span> : null}
        </>
      )}
    </span>
  );
}
