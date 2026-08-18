import { ChevronDown, ChevronRight, FileText, Folder } from 'lucide-react';
import { useCallback, useState } from 'react';

import { cn } from '@/lib/utils';
import { useWorkbenchTree } from '@/queries/workbench';

import type { TreeSource } from '../params';

// 文件树（提案 1012）：懒加载展开，每层一次 tree 接口请求。
// 受控组件（供 1013 diff 面板复用）：selectedFile/onPick 由调用方管理。
// filter 生效时只显示集合内文件（及其祖先目录）——差异模式用
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
  // 展开状态提升到树级统一管理（按目录相对路径），子节点无状态渲染。
  // 之前各 DirNode 自持 useState，实测出现「点一个目录全部联动开/关」的异常，
  // 提升后状态与节点实例生命周期解耦，也天然在数据刷新后保持。
  const [expandedSet, setExpandedSet] = useState<ReadonlySet<string>>(() => new Set(['']));
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
  return (
    <div className="flex h-full flex-col overflow-y-auto p-1 text-xs">
      <DirNode
        path={path}
        source={source}
        dir=""
        depth={0}
        selectedFile={selectedFile}
        onPick={onPick}
        filter={filter}
        expandedSet={expandedSet}
        onToggle={toggle}
      />
    </div>
  );
}

function DirNode({
  path,
  source,
  dir,
  depth,
  selectedFile,
  onPick,
  filter,
  expandedSet,
  onToggle,
}: {
  path: string;
  source: TreeSource;
  dir: string;
  depth: number;
  selectedFile: string;
  onPick: (file: string) => void;
  filter: Set<string> | null;
  expandedSet: ReadonlySet<string>;
  onToggle: (dir: string) => void;
}) {
  const tree = useWorkbenchTree(path, source, dir);

  if (tree.isPending) {
    return (
      <div className="py-1 pl-3 text-muted-foreground" style={{ paddingLeft: depth * 12 + 12 }}>
        加载中…
      </div>
    );
  }
  if (tree.isError) {
    return (
      <div className="py-1 text-destructive" style={{ paddingLeft: depth * 12 + 12 }}>
        {tree.error.message}
      </div>
    );
  }

  // 差异过滤：文件须在集合内；目录须有集合内文件位于其下
  const entries = (tree.data ?? []).filter((e) => {
    if (!filter) return true;
    const rel = dir ? `${dir}/${e.name}` : e.name;
    return e.dir ? hasFileUnder(filter, rel) : filter.has(rel);
  });
  return (
    <>
      {entries.map((e) => {
        const rel = dir ? `${dir}/${e.name}` : e.name;
        if (e.dir) {
          // 行内必须查「子目录自己」的展开状态（rel），不能用本节点的 expanded——
          // 之前就是这里错位：父级的 expanded 控制了所有子行的 chevron 与挂载，
          // 表现为「点一个目录全部联动开/关」
          const childExpanded = expandedSet.has(rel);
          return (
            <div key={rel}>
              <button
                type="button"
                className="flex w-full items-center gap-1 rounded px-1 py-0.5 text-left hover:bg-accent"
                style={{ paddingLeft: depth * 12 + 4 }}
                onClick={() => onToggle(rel)}
              >
                {childExpanded ? (
                  <ChevronDown className="size-3 shrink-0" />
                ) : (
                  <ChevronRight className="size-3 shrink-0" />
                )}
                <Folder className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="truncate">{e.name}</span>
              </button>
              {childExpanded ? (
                <DirNode
                  path={path}
                  source={source}
                  dir={rel}
                  depth={depth + 1}
                  selectedFile={selectedFile}
                  onPick={onPick}
                  filter={filter}
                  expandedSet={expandedSet}
                  onToggle={onToggle}
                />
              ) : null}
            </div>
          );
        }
        const selected = selectedFile === rel;
        return (
          <button
            key={rel}
            type="button"
            title={rel}
            className={cn(
              'flex w-full items-center gap-1 rounded px-1 py-0.5 text-left hover:bg-accent',
              selected && 'bg-primary/15 font-medium',
            )}
            style={{ paddingLeft: depth * 12 + 16 }}
            onClick={() => onPick(rel)}
          >
            <FileText
              className={cn('size-3.5 shrink-0', e.ignored ? 'text-muted-foreground/50' : 'text-muted-foreground')}
            />
            <span className={cn('truncate', e.ignored && 'text-muted-foreground/60')}>{e.name}</span>
          </button>
        );
      })}
      {entries.length === 0 ? (
        <div className="py-1 text-muted-foreground" style={{ paddingLeft: depth * 12 + 12 }}>
          （{filter ? '无变更文件' : '空目录'}）
        </div>
      ) : null}
    </>
  );
}

function hasFileUnder(filter: Set<string>, dir: string): boolean {
  const prefix = dir + '/';
  for (const f of filter) {
    if (f.startsWith(prefix)) return true;
  }
  return false;
}
