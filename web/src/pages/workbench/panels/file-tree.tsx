import { ChevronDown, ChevronRight, FileText, Folder } from 'lucide-react';
import { useState } from 'react';

import { cn } from '@/lib/utils';
import { useWorkbenchTree } from '@/queries/workbench';

import type { TreeSource } from '../params';

// 文件树（提案 1012）：懒加载展开，每层一次 tree 接口请求。
// 受控组件（供 1013 diff 面板复用）：selectedFile/onPick 由调用方管理。
export function FileTree({
  path,
  source,
  selectedFile,
  onPick,
}: {
  path: string;
  source: TreeSource;
  selectedFile: string;
  onPick: (file: string) => void;
}) {
  return (
    <div className="flex h-full flex-col overflow-y-auto p-1 text-xs">
      <DirNode path={path} source={source} dir="" depth={0} selectedFile={selectedFile} onPick={onPick} />
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
}: {
  path: string;
  source: TreeSource;
  dir: string;
  depth: number;
  selectedFile: string;
  onPick: (file: string) => void;
}) {
  const [expanded, setExpanded] = useState(depth === 0);
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

  const entries = tree.data ?? [];
  return (
    <>
      {entries.map((e) => {
        const rel = dir ? `${dir}/${e.name}` : e.name;
        if (e.dir) {
          return (
            <div key={rel}>
              <button
                type="button"
                className="flex w-full items-center gap-1 rounded px-1 py-0.5 text-left hover:bg-accent"
                style={{ paddingLeft: depth * 12 + 4 }}
                onClick={() => setExpanded((v) => !v)}
              >
                {expanded ? <ChevronDown className="size-3 shrink-0" /> : <ChevronRight className="size-3 shrink-0" />}
                <Folder className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="truncate">{e.name}</span>
              </button>
              {expanded ? (
                <DirNode
                  path={path}
                  source={source}
                  dir={rel}
                  depth={depth + 1}
                  selectedFile={selectedFile}
                  onPick={onPick}
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
          （空目录）
        </div>
      ) : null}
    </>
  );
}
