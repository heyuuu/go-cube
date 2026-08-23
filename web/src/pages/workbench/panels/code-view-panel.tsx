import { useState } from 'react';
import { useSearchParams } from 'react-router';

import { Badge } from '@/components/ui/badge';
import { useWorkbenchChanges, useWorkbenchFile, useWorkbenchFileDiff, useWorkbenchTree } from '@/queries/workbench';
import { sourceLabel, writeFileParam, type WorkbenchParams } from '../params';

import { FileContentArea, useFileEditing, type ContentMode } from './file-content';
import { SourcePanelShell, useTreePanePrefs } from './tree-pane';

// 代码阅读面板（提案 1012）：单选 TreeSource 的文件浏览。树列 + 内容区均为公共组件
// （SourcePanelShell / FileContentArea），本文件只剩数据编排与面板专属徽标。
// 内容区支持 单文件（默认，worktree 源可确认式轻编辑）与 diff（vs 父提交/HEAD）双模式。

// 文件树偏好键前缀（视图/范围/宽度，见 tree-pane.tsx 的 useTreePanePrefs）
const TREE_PREFS_KEY = 'cube.workbench.codetree';

export function CodeViewPanel({ params }: { params: WorkbenchParams }) {
  const { path, source } = params;
  const [searchParams, setSearchParams] = useSearchParams();
  const file = searchParams.get('file') ?? '';

  const treePrefs = useTreePanePrefs(TREE_PREFS_KEY, 'all');
  const [mode, setMode] = useState<ContentMode>('file');

  // 与 FileTree 同 key 的树数据（react-query 去重复用，无额外请求）：
  // 用于回退判定——切换 commit/分支后 file 参数可能指向新源里不存在的文件
  const tree = useWorkbenchTree(path, source!);
  const treeList = tree.data?.list ?? null;
  // 回退策略：file 不在当前源中 → 根目录 README.md → 都没有则空并提示。
  // 只做显示层回退不改写 URL——file 参数指向用户最后的选择，源切换是临时浏览上下文
  const fileMissing = !!treeList && !!file && !treeList.includes(file);
  const activeFile = !fileMissing ? file : treeList?.includes('README.md') ? 'README.md' : '';

  const content = useWorkbenchFile(path, source!, activeFile);
  // diff 模式：相对基准（ref/commit vs 父提交、worktree vs HEAD），左源缺省
  const fileDiff = useWorkbenchFileDiff(path, null, source!, activeFile, mode === 'diff');
  // changes 全量模式也拉：行级统计（+N -N/琥珀色）在两种范围下都展示
  const changes = useWorkbenchChanges(path, source!, true);
  const editing = useFileEditing(path, source!, activeFile, content.data?.content ?? '');

  const diffFilter =
    treePrefs.scope === 'diff' && changes.data ? new Set((changes.data.list ?? []).map((e) => e.path)) : null;
  const diffStats = changes.data
    ? new Map(
        (changes.data.list ?? []).map((e) => [
          e.path,
          { adds: e.adds, dels: e.dels, binary: e.binary, status: e.status, oldPath: e.oldPath || undefined },
        ]),
      )
    : null;

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

  if (!source) return null;

  const headerLeading = (
    <>
      <Badge variant="secondary" className="max-w-48 shrink-0" title={source.id}>
        <span className="text-[10px] text-muted-foreground">
          {source.type === 'worktree' ? '工作副本' : source.type === 'ref' ? '分支' : '提交'}
        </span>
        <span className="ml-1 truncate font-mono">{sourceLabel(source)}</span>
      </Badge>
      <span className="truncate font-medium">{activeFile || '未选择文件'}</span>
    </>
  );

  return (
    <SourcePanelShell
      prefs={treePrefs}
      path={path}
      treeSource={source}
      selectedFile={activeFile}
      onPick={pickFile}
      diffFilter={diffFilter}
      stats={diffStats}
      statsPending={changes.isPending}
    >
      <FileContentArea
        mode={mode}
        onMode={setMode}
        file={activeFile}
        fileMissing={fileMissing}
        canEdit={source.type === 'worktree'}
        editing={editing}
        contentQuery={content}
        fileDiffQuery={fileDiff}
        headerLeading={headerLeading}
      />
    </SourcePanelShell>
  );
}
