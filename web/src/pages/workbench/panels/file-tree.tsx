import { ChevronDown, ChevronRight, FileText, Folder } from 'lucide-react';
import { useCallback, useMemo, useState } from 'react';

import { TreeToolbar } from '@/components/tree-toolbar';
import { buildFileTree, flattenFileTree, type FileTreeNode } from '@/lib/tree';
import { cn } from '@/lib/utils';
import { useWorkbenchTree } from '@/queries/workbench';

import type { TreeSource } from '../params';

// 文件树：一次全量拉取 git 管理的文件清单（扁平相对路径），前端用 lib/tree 组树——
// 与 md 页同一套组树/拍平/折叠链逻辑。数据全在内存，展开/折叠/全部展开均为本地状态切换。
// 受控组件（供代码阅读/diff 面板复用）：selectedFile/onPick 由调用方管理。
// filter 生效时只显示集合内文件（及其祖先目录）——差异模式用。
export function FileTree({
  path,
  source,
  selectedFile,
  onPick,
  filter,
}: {
  path: string;
  source: TreeSource;
  selectedFile: string;
  onPick: (file: string) => void;
  filter: Set<string> | null;
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

  // 可见行：flatten 后按差异过滤——文件须在集合内，目录须有集合内文件位于其下。
  // 根行（isRoot）不渲染，面板标题已提供上下文
  const rows = useMemo(() => {
    if (!root) return [];
    return flattenFileTree(root, (p) => expandedSet.has(p)).filter((r) => {
      if (r.isRoot) return false;
      if (!filter) return true;
      return r.node.kind === 'dir' ? hasFileUnder(filter, r.node.path) : filter.has(r.node.path);
    });
  }, [root, expandedSet, filter]);

  return (
    <div className="flex h-full flex-col">
      <TreeToolbar onExpandAll={expandAll} onCollapseAll={collapseAll} />
      <div className="min-h-0 flex-1 overflow-y-auto p-1 text-xs">
        {tree.isPending ? (
          <div className="py-1 pl-3 text-muted-foreground">加载中…</div>
        ) : tree.isError ? (
          <div className="py-1 pl-3 text-destructive">{tree.error.message}</div>
        ) : rows.length === 0 ? (
          <div className="py-1 pl-3 text-muted-foreground">（{filter ? '无变更文件' : '空目录'}）</div>
        ) : (
          rows.map(({ node, depth, expanded }) =>
            node.kind === 'dir' ? (
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
                style={{ paddingLeft: depth * 12 + 16 }}
                onClick={() => onPick(node.path)}
              >
                <FileText className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="truncate">{node.name}</span>
              </button>
            ),
          )
        )}
      </div>
    </div>
  );
}

function hasFileUnder(filter: Set<string>, dir: string) {
  const prefix = dir + '/';
  for (const f of filter) {
    if (f.startsWith(prefix)) return true;
  }
  return false;
}
