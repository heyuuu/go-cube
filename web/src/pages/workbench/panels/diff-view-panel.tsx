import { ArrowRight, FilePlus2, FileX2, FilePen, ArrowLeftRight } from 'lucide-react';
import { useState } from 'react';
import { useSearchParams } from 'react-router';

import type { components } from '@/api/schema';
import { ErrorBanner } from '@/components/error-banner';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { useWorkbenchDiff, useWorkbenchFileDiff } from '@/queries/workbench';

import { sourceLabel, type TreeSource, type WorkbenchParams } from '../params';

// diff 面板（提案 1013）：双 TreeSource 对比（Beyond Compare 级）。
// 目录级 = 变更文件列表（状态过滤 / ignored 开关 / 路径过滤）；
// 文件级 = side-by-side 双栏 hunks。方向约定：left = 基准（old），right = 对比（new）。
// 筛选项放组件内 state 不进 URL（避免参数爆炸）；选中文件复用 file 参数。

const STATUS_ICONS: Record<string, typeof FilePen> = {
  added: FilePlus2,
  deleted: FileX2,
  modified: FilePen,
  renamed: ArrowLeftRight,
};

const STATUS_LABELS: Record<string, string> = {
  added: '新增',
  deleted: '删除',
  modified: '修改',
  renamed: '重命名',
};

export function DiffViewPanel({ params }: { params: WorkbenchParams }) {
  const { path, left, right } = params;
  const [searchParams, setSearchParams] = useSearchParams();
  const file = searchParams.get('file') ?? '';

  const [statusOn, setStatusOn] = useState<Record<string, boolean>>({});
  const [showIgnored, setShowIgnored] = useState(false);
  const [pathPrefix, setPathPrefix] = useState('');

  const statusFilter = Object.entries(statusOn)
    .filter(([, on]) => on)
    .map(([s]) => s)
    .join(',');

  const diff = useWorkbenchDiff(path, left ?? EMPTY_SOURCE, right ?? EMPTY_SOURCE, {
    showIgnored,
    statusFilter,
    pathPrefix,
  });
  const fileDiff = useWorkbenchFileDiff(path, left ?? EMPTY_SOURCE, right ?? EMPTY_SOURCE, file);
  if (!left || !right) return null;

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
      <div className="flex w-80 shrink-0 flex-col border-r border-border">
        <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-border px-2 py-1.5">
          {(['added', 'deleted', 'modified', 'renamed'] as const).map((s) => (
            <label key={s} className="flex items-center gap-1 text-xs text-muted-foreground">
              <Checkbox
                checked={statusOn[s] ?? false}
                onCheckedChange={() => setStatusOn((prev) => ({ ...prev, [s]: !prev[s] }))}
              />
              {STATUS_LABELS[s]}
            </label>
          ))}
          <label className="flex items-center gap-1 text-xs text-muted-foreground">
            <Checkbox checked={showIgnored} onCheckedChange={() => setShowIgnored((v) => !v)} />
            含 ignored
          </label>
        </div>
        <div className="shrink-0 border-b border-border px-2 py-1.5">
          <Input
            value={pathPrefix}
            onChange={(e) => setPathPrefix(e.target.value)}
            placeholder="路径过滤…"
            className="h-6 text-xs"
          />
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-1">
          {diff.isPending ? (
            <div className="p-2 text-xs text-muted-foreground">对比中…</div>
          ) : diff.isError ? (
            <ErrorBanner message={diff.error.message} />
          ) : (
            <>
              <div className="flex items-center gap-1 px-1 pb-1 text-[11px] text-muted-foreground">
                {diff.data?.mode === 'fs' ? '文件系统扫描模式' : 'git 模式'} · {diff.data?.list?.length ?? 0} 项
              </div>
              {(diff.data?.list ?? []).map((e) => {
                const Icon = STATUS_ICONS[e.status] ?? FilePen;
                return (
                  <button
                    key={e.path}
                    type="button"
                    title={e.oldPath ? `${e.oldPath} → ${e.path}` : e.path}
                    onClick={() => pickFile(e.path)}
                    className={cn(
                      'flex w-full items-center gap-1.5 rounded px-1.5 py-0.5 text-left text-xs hover:bg-accent',
                      file === e.path && 'bg-primary/15 font-medium',
                    )}
                  >
                    <Icon
                      className={cn(
                        'size-3.5 shrink-0',
                        e.status === 'added' && 'text-emerald-500',
                        e.status === 'deleted' && 'text-red-500',
                        (e.status === 'modified' || e.status === 'renamed') && 'text-amber-500',
                      )}
                    />
                    <span className="truncate">{e.path}</span>
                    {e.oldPath ? (
                      <span className="shrink-0 text-[10px] text-muted-foreground">← {e.oldPath}</span>
                    ) : null}
                  </button>
                );
              })}
              {(diff.data?.list ?? []).length === 0 ? (
                <div className="p-2 text-xs text-muted-foreground">无差异</div>
              ) : null}
            </>
          )}
        </div>
      </div>
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
