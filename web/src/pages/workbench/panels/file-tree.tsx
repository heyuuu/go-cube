import { ChevronDown, ChevronRight, FileText, Folder, List, ListTree } from 'lucide-react';
import { useCallback, useMemo, useState } from 'react';

import { TreeToolbar } from '@/components/tree-toolbar';
import { Button } from '@/components/ui/button';
import { buildFileTree, flattenFileTree, type FileTreeNode } from '@/lib/tree';
import { cn } from '@/lib/utils';
import { useWorkbenchTree } from '@/queries/workbench';

import type { TreeSource } from '../params';

// 文件树：一次全量拉取 git 管理的文件清单（扁平相对路径），前端用 lib/tree 组树——
// 与 md 页同一套组树/拍平/折叠链逻辑。数据全在内存，展开/折叠/全部展开均为本地状态切换。
// 受控组件（供代码阅读/diff 面板复用）：selectedFile/onPick 由调用方管理。
// filter 生效时只显示集合内文件（及其祖先目录）——差异模式用。
// viewMode：树形（目录可展开）/ 平摊（每行一个文件，显示相对根目录的完整路径），
// scope（全量/差异文件）与模式切换按钮渲染在树工具条（extra），状态由调用方持有。
export function FileTree({
  path,
  source,
  selectedFile,
  onPick,
  filter,
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
  viewMode: 'tree' | 'flat';
  onViewMode: (m: 'tree' | 'flat') => void;
  scope: 'all' | 'diff';
  onScope: (s: 'all' | 'diff') => void;
  scopePending: boolean;
}) {
  // 展开状态提升到树级统一管理（按目录相对路径），行组件无状态渲染
  const [expandedSet, setExpandedSet] = useState<ReadonlySet<string>>(() => new Set(['']));
  const tree = useWorkbenchTree(path, source);

  const root = useMemo(
    () => (tree.data?.list?.length ? buildFileTree('', tree.data.list) : null),
    [tree.data],
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
    const next = new Set<string>();
    const walk = (n: FileTreeNode) => {
      if (n.children.length > 0) {
        next.add(n.path);
        n.children.forEach(walk);
      }
    };
    root.children.forEach(walk);
    setExpandedSet(next);
  }, [root]);

  const collapseAll = useCallback(() => setExpandedSet(new Set([''])), []);

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
        extra={
          <>
            <Button
              variant="ghost"
              size="icon-sm"
              className="text-muted-foreground"
              onClick={() => onViewMode(flat ? 'tree' : 'flat')}
              title={flat ? '切换为树形目录' : '切换为平摊列表（显示完整路径）'}
              aria-label={flat ? '切换为树形目录' : '切换为平摊列表'}
            >
              {flat ? <ListTree className="size-3.5" /> : <List className="size-3.5" />}
            </Button>
            <div className="ml-auto flex overflow-hidden rounded-md border border-border text-[10px]">
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
          </>
        }
      />
      <div className="min-h-0 flex-1 overflow-y-auto p-1 text-xs">
        {tree.isPending ? (
          <div className="py-1 pl-3 text-muted-foreground">加载中…</div>
        ) : tree.isError ? (
          <div className="py-1 pl-3 text-destructive">{tree.error.message}</div>
        ) : rows.length === 0 ? (
          <div className="py-1 pl-3 text-muted-foreground">（{filter ? '无变更文件' : '空目录'}）</div>
        ) : (
          rows.map(({ node, depth, expanded }) =>
            node.kind === 'dir' && !flat ? (
              <button
                key={node.path}
                type="button"
                className="flex w-full items-center gap-1 rounded px-1 py-0.5 text-left hover:bg-accent"
                style={{ paddingLeft: depth * 12 + 4 }}
                onClick={() => toggle(node.path)}
              >
                {expanded ? (
                  <ChevronDown className="size-3 shrink-0" />
                ) : (
                  <ChevronRight className="size-3 shrink-0" />
                )}
                <Folder className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="truncate">{node.name}</span>
              </button>
            ) : (
              <button
                key={node.path}
                type="button"
                title={node.path}
                className={cn(
                  'flex w-full items-center gap-1 rounded px-1 py-0.5 text-left hover:bg-accent',
                  selectedFile === node.path && 'bg-primary/15 font-medium',
                )}
                style={{ paddingLeft: flat ? 6 : depth * 12 + 16 }}
                onClick={() => onPick(node.path)}
              >
                <FileText className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="truncate">{flat ? node.path : node.name}</span>
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
