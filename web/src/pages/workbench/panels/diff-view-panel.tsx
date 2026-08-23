import { ArrowRight } from 'lucide-react';
import { useState } from 'react';
import { useSearchParams } from 'react-router';

import type { components } from '@/api/schema';
import { ErrorBanner } from '@/components/error-banner';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { useWorkbenchDiff, useWorkbenchFileDiff } from '@/queries/workbench';

import { sourceLabel, type TreeSource, type WorkbenchParams } from '../params';

import { FileTreePane, useTreePanePrefs } from './tree-pane';

// diff 面板（提案 1013）：双 TreeSource 对比（Beyond Compare 级）。
// 目录级 = 变更文件树（复用 code 面板的 FileTree：组树/着色/树形平摊/宽度拖拽）；
// 文件级 = side-by-side 双栏 hunks。方向约定：left = 基准（old），right = 对比（new）。
// 状态多选/含 ignored 筛选已移除（着色直读、变更集不大）；路径搜索为前端本地子串过滤。
// 筛选项不进 URL；选中文件复用 file 参数。

// 树偏好 localStorage 键前缀（与 code 面板各自独立，见 tree-pane.tsx）
const TREE_PREFS_KEY = 'cube.workbench.difftree';

export function DiffViewPanel({ params }: { params: WorkbenchParams }) {
  const { path, left, right } = params;
  const [searchParams, setSearchParams] = useSearchParams();
  const file = searchParams.get('file') ?? '';

  const [query, setQuery] = useState('');
  const treePrefs = useTreePanePrefs(TREE_PREFS_KEY, 'diff');

  const diff = useWorkbenchDiff(path, left ?? EMPTY_SOURCE, right ?? EMPTY_SOURCE);
  const fileDiff = useWorkbenchFileDiff(path, left ?? EMPTY_SOURCE, right ?? EMPTY_SOURCE, file);
  if (!left || !right) return null;

  // 本地路径子串搜索（只在差异范围下提供——全量树行数多，搜索语义另做）；
  // 状态不提供筛选项——着色直读、变更集通常不大
  const q = query.trim();
  const entries = (diff.data?.list ?? []).filter(
    (e) => !q || e.path.includes(q) || (e.oldPath ?? '').includes(q),
  );
  const filterSet = new Set(entries.map((e) => e.path));
  const statsMap = new Map(
    (diff.data?.list ?? []).map((e) => [e.path, { adds: 0, dels: 0, status: e.status, oldPath: e.oldPath || undefined }]),
  );

  const pickFile = (f: string) => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.set('file', f);
        return next;
      },
      { replace: true },
    );
  };

  return (
    <div className="flex h-full min-h-0">
      {diff.isError ? (
        <div className="w-80 shrink-0">
          <ErrorBanner message={diff.error.message} />
        </div>
      ) : (
        <FileTreePane
          prefs={treePrefs}
          above={
            treePrefs.scope === 'diff' ? (
              <div className="shrink-0 border-b border-border px-2 py-1.5">
                <Input
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder="搜索路径（子串）…"
                  className="h-6 text-xs"
                />
              </div>
            ) : null
          }
          toolbarExtra={
            diff.data ? (
              <span className="text-[10px] text-muted-foreground">
                {diff.data.mode === 'fs' ? '文件系统扫描' : 'git 模式'} · {entries.length} 项
              </span>
            ) : null
          }
          path={path}
          source={right} // 右侧 = 对比的「新」侧：全量树的现状、目录状态推导基准
          selectedFile={file}
          onPick={pickFile}
          filter={treePrefs.scope === 'diff' ? filterSet : null}
          stats={statsMap}
          statsPending={diff.isPending}
        />
      )}
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex shrink-0 items-center gap-2 border-b border-border px-3 py-1.5 text-xs">
          <Badge variant="secondary">{sourceLabel(left)}</Badge>
          <ArrowRight className="size-3 text-muted-foreground" />
          <Badge variant="secondary">{sourceLabel(right)}</Badge>
          <span className="ml-2 truncate font-medium">{file || '未选择文件'}</span>
        </div>
        {!file ? (
          <div className="flex flex-1 items-center justify-center text-xs text-muted-foreground">
            在左侧选择一个变更文件查看双栏对比
          </div>
        ) : fileDiff.isPending ? (
          <div className="p-3 text-xs text-muted-foreground">计算 diff…</div>
        ) : fileDiff.isError ? (
          <ErrorBanner message={fileDiff.error.message} />
        ) : fileDiff.data?.binary ? (
          <div className="flex flex-1 items-center justify-center text-xs text-muted-foreground">二进制文件差异</div>
        ) : (
          <SideBySideHunks hunks={fileDiff.data?.hunks ?? []} />
        )}
      </div>
    </div>
  );
}

// side-by-side 双栏渲染：del 进左栏、add 进右栏、ctx 两侧同步；连续 del/add 块按行配对
function SideBySideHunks({ hunks }: { hunks: components['schemas']['Hunk'][] }) {
  if (hunks.length === 0) {
    return <div className="flex flex-1 items-center justify-center text-xs text-muted-foreground">两侧内容一致</div>;
  }
  return (
    <div className="min-h-0 flex-1 overflow-auto font-mono text-[12px] leading-5">
      {hunks.map((h, i) => (
        <div key={i}>
          <div className="bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
            @@ -{h.oldStart},{h.oldCount} +{h.newStart},{h.newCount} @@
          </div>
          <HunkRows hunk={h} />
        </div>
      ))}
    </div>
  );
}

function HunkRows({ hunk }: { hunk: components['schemas']['Hunk'] }) {
  // 连续 del 块与 add 块逐行配对（左删右增同行对照），剩余各自单侧展示
  type Row = { left?: string; right?: string; kind: 'del' | 'add' | 'pair' };
  const rows: Row[] = [];
  let pendingDels: string[] = [];
  const flush = () => {
    for (const d of pendingDels) rows.push({ left: d, kind: 'del' });
    pendingDels = [];
  };
  for (const line of hunk.lines ?? []) {
    if (line.kind === 'ctx') {
      flush();
      rows.push({ left: line.text, right: line.text, kind: 'pair' });
    } else if (line.kind === 'del') {
      pendingDels.push(line.text);
    } else {
      const paired = pendingDels.shift();
      if (paired !== undefined) {
        rows.push({ left: paired, right: line.text, kind: 'pair' });
      } else {
        rows.push({ right: line.text, kind: 'add' });
      }
    }
  }
  flush();

  return (
    <table className="w-full table-fixed border-collapse">
      <tbody>
        {rows.map((r, i) => {
          const changed = r.kind !== 'pair';
          const leftChanged = changed && r.left !== undefined && (r.right === undefined || r.kind === 'del');
          const rightChanged = changed && r.right !== undefined && (r.left === undefined || r.kind === 'add');
          return (
            <tr key={i} className="align-top">
              <td
                className={cn(
                  'w-1/2 whitespace-pre-wrap break-all border-r border-border px-2',
                  leftChanged && 'bg-red-500/10 text-red-600 dark:text-red-400',
                )}
              >
                {r.left ?? ''}
              </td>
              <td
                className={cn(
                  'w-1/2 whitespace-pre-wrap break-all px-2',
                  rightChanged && 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
                )}
              >
                {r.right ?? ''}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

// 双源缺一时的占位（hooks 必须无条件调用；空源走 enabled=false）
const EMPTY_SOURCE: TreeSource = { type: 'ref', id: '' };
