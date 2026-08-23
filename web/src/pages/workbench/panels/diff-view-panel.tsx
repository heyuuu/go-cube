import { ArrowRight } from 'lucide-react';
import { useState } from 'react';
import { useSearchParams } from 'react-router';

import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { useWorkbenchDiff, useWorkbenchFile, useWorkbenchFileDiff } from '@/queries/workbench';
import { sourceLabel, writeFileParam, type TreeSource, type WorkbenchParams } from '../params';

import { FileContentArea, useFileEditing, type ContentMode } from './file-content';
import { SourcePanelShell, useTreePanePrefs } from './tree-pane';

// diff 面板（提案 1013）：双 TreeSource 对比（Beyond Compare 级）。
// 树列 + 内容区均为公共组件（SourcePanelShell / FileContentArea），本文件只剩数据编排
// 与左右源徽标。方向约定：left = 基准（old），right = 对比（new）。
// 内容区支持 diff（默认，双栏 hunks）与 单文件（右侧源，worktree 时可轻编辑）双模式。
// 路径搜索为前端本地子串过滤（变更清单一次全量返回）；筛选项不进 URL；选中文件复用 file 参数。

// 树偏好 localStorage 键前缀（与 code 面板各自独立，见 tree-pane.tsx）
const TREE_PREFS_KEY = 'cube.workbench.difftree';

export function DiffViewPanel({ params }: { params: WorkbenchParams }) {
  const { path, left, right } = params;
  const [searchParams, setSearchParams] = useSearchParams();
  const file = searchParams.get('file') ?? '';

  const [query, setQuery] = useState('');
  const treePrefs = useTreePanePrefs(TREE_PREFS_KEY, 'diff');
  const [mode, setMode] = useState<ContentMode>('diff');

  const diff = useWorkbenchDiff(path, left ?? EMPTY_SOURCE, right ?? EMPTY_SOURCE);
  const fileDiff = useWorkbenchFileDiff(path, left ?? null, right ?? EMPTY_SOURCE, file, mode === 'diff');
  // 单文件模式的数据：右侧（新）源；worktree 源因此获得编辑能力
  const content = useWorkbenchFile(path, right ?? EMPTY_SOURCE, file);
  const editing = useFileEditing(path, right ?? EMPTY_SOURCE, file, content.data?.content ?? '');

  // 本地路径子串搜索（只在差异范围下提供——全量树行数多，搜索语义另做）
  const q = query.trim();
  const entries = (diff.data?.list ?? []).filter(
    (e) => !q || e.path.includes(q) || (e.oldPath ?? '').includes(q),
  );
  const filterSet = new Set(entries.map((e) => e.path));
  const statsMap = new Map(
    (diff.data?.list ?? []).map((e) => [e.path, { adds: 0, dels: 0, status: e.status, oldPath: e.oldPath || undefined }]),
  );

  const pickFile = (f: string) =>
    editing.guardSwitch(() =>
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          writeFileParam(next, f);
          return next;
        },
        { replace: true },
      ),
    );

  if (!left || !right) return null;

  const headerLeading = (
    <>
      <Badge variant="secondary">{sourceLabel(left)}</Badge>
      <ArrowRight className="size-3 text-muted-foreground" />
      <Badge variant="secondary">{sourceLabel(right)}</Badge>
      <span className="ml-2 truncate font-medium">{file || '未选择文件'}</span>
    </>
  );

  return (
    <SourcePanelShell
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
      treeSource={right} // 右侧 = 对比的「新」侧：全量树的现状、目录状态推导基准
      selectedFile={file}
      onPick={pickFile}
      diffFilter={treePrefs.scope === 'diff' ? filterSet : null}
      stats={statsMap}
      statsPending={diff.isPending}
    >
      <FileContentArea
        mode={mode}
        onMode={setMode}
        file={file}
        canEdit={right.type === 'worktree'}
        editing={editing}
        contentQuery={content}
        fileDiffQuery={fileDiff}
        headerLeading={headerLeading}
      />
    </SourcePanelShell>
  );
}

// 双源缺一时的占位（hooks 必须无条件调用；空源走 enabled=false）
const EMPTY_SOURCE: TreeSource = { type: 'ref', id: '' };
