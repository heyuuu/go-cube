import { ArrowRight } from 'lucide-react';
import { useState } from 'react';
import { useSearchParams } from 'react-router';

import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import {
  useWorkbenchChanges,
  useWorkbenchDiff,
  useWorkbenchFile,
  useWorkbenchFileDiff,
  useWorkbenchTree,
} from '@/queries/workbench';
import { sourceLabel, writeFileParam, type TreeSource, type WorkbenchParams } from '../params';

import { FileContentArea, useFileEditing, type ContentMode } from './file-content';
import { SourcePanelShell, useTreePanePrefs } from './tree-pane';

// 内容面板（code + diff 合并）：统一为「source [+ base]」视图模型——
//   单选：source = 选中目标，base = 空 → diff 模式与父提交/HEAD 比（worktree 则 vs HEAD）；
//   双选：source = 右侧（新），base = 左侧（基准）→ diff 模式与 base 比。
// URL 参数仍用 source / left+right（交互迁移另做），此处映射：
//   viewSource = params.source ?? params.right；viewBase = 双选时的 params.left。
// 树列/内容区/编辑流均为公共组件（SourcePanelShell / FileContentArea / useFileEditing）。

// 树偏好键前缀（视图/范围/宽度，见 tree-pane.tsx 的 useTreePanePrefs）
const TREE_PREFS_KEY = 'cube.workbench.contenttree';

export function ContentViewPanel({ params }: { params: WorkbenchParams }) {
  const { path } = params;
  const [searchParams, setSearchParams] = useSearchParams();
  const file = searchParams.get('file') ?? '';

  const viewSource = params.source ?? params.right ?? null;
  const viewBase = params.source ? null : (params.left ?? null);
  // 空源占位（hooks 必须无条件调用；空源走 enabled=false）
  const src = viewSource ?? EMPTY_SOURCE;

  const treePrefs = useTreePanePrefs(TREE_PREFS_KEY, 'all');
  // 内容模式默认随双选态：双选进 diff、单选进单文件；用户手动切换后保留，
  // 双选↔单选切换时重置（render 期 setState 的派生重置模式）
  const baseKey = viewBase ? `${viewBase.type}:${viewBase.id}` : '';
  const [mode, setMode] = useState<ContentMode>(viewBase ? 'diff' : 'file');
  const [prevBaseKey, setPrevBaseKey] = useState(baseKey);
  if (prevBaseKey !== baseKey) {
    setPrevBaseKey(baseKey);
    setMode(viewBase ? 'diff' : 'file');
  }

  // 与 FileTree 同 key 的树数据（react-query 去重复用）：单选时做 file 回退判定——
  // 切换 commit/分支后 file 参数可能指向新源里不存在的文件。只做显示层回退不改写
  // URL——file 参数指向用户最后的选择，源切换是临时浏览上下文
  const tree = useWorkbenchTree(path, src);
  const treeList = tree.data?.list ?? null;
  const fileMissing = !!treeList && !!file && !treeList.includes(file);
  const activeFile = !fileMissing ? file : treeList?.includes('README.md') ? 'README.md' : '';

  const content = useWorkbenchFile(path, src, activeFile);
  const fileDiff = useWorkbenchFileDiff(path, viewBase, src, activeFile, mode === 'diff');

  // 差异范围的变更清单：base 空 = 相对基准（changes，含 untracked 与行级统计）；
  // base 有 = 两源对比（diff）。两者同构，行级统计后端均已注入
  const changes = useWorkbenchChanges(path, src, !viewBase);
  const diff = useWorkbenchDiff(path, viewBase ?? EMPTY_SOURCE, src); // 空 base 时 enabled=false（left.id 为空）
  const changeList = viewBase ? (diff.data?.list ?? []) : (changes.data?.list ?? []);
  const listPending = viewBase ? diff.isPending : changes.isPending;

  const statsMap = new Map(
    changeList.map((e) => [e.path, { adds: e.adds, dels: e.dels, binary: e.binary, status: e.status, oldPath: e.oldPath || undefined }]),
  );
  const editing = useFileEditing(path, src, activeFile, content.data?.content ?? '');

  // 本地路径子串搜索（只在差异范围下提供）
  const [query, setQuery] = useState('');
  const q = query.trim();
  const entries = changeList.filter((e) => !q || e.path.includes(q) || (e.oldPath ?? '').includes(q));
  const diffFilter = treePrefs.scope === 'diff' ? new Set(entries.map((e) => e.path)) : null;

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

  if (!viewSource) return null;

  const headerLeading = viewBase ? (
    <>
      <Badge variant="secondary">{sourceLabel(viewBase)}</Badge>
      <ArrowRight className="size-3 text-muted-foreground" />
      <Badge variant="secondary" className="max-w-48" title={viewSource.id}>
        <span className="text-[10px] text-muted-foreground">
          {viewSource.type === 'worktree' ? '工作副本' : viewSource.type === 'ref' ? '分支' : '提交'}
        </span>
        <span className="ml-1 truncate font-mono">{sourceLabel(viewSource)}</span>
      </Badge>
      <span className="truncate font-medium">{activeFile || '未选择文件'}</span>
    </>
  ) : (
    <>
      <Badge variant="secondary" className="max-w-48 shrink-0" title={viewSource.id}>
        <span className="text-[10px] text-muted-foreground">
          {viewSource.type === 'worktree' ? '工作副本' : viewSource.type === 'ref' ? '分支' : '提交'}
        </span>
        <span className="ml-1 truncate font-mono">{sourceLabel(viewSource)}</span>
      </Badge>
      <span className="truncate font-medium">{activeFile || '未选择文件'}</span>
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
        !listPending && changeList.length >= 0 && (viewBase || treePrefs.scope === 'diff') ? (
          <span className="text-[10px] text-muted-foreground">
            {viewBase
              ? `${diff.data?.mode === 'fs' ? '文件系统扫描' : 'git 模式'} · ${entries.length} 项`
              : `${entries.length} 项变更`}
          </span>
        ) : null
      }
      path={path}
      treeSource={src}
      selectedFile={activeFile}
      onPick={pickFile}
      diffFilter={diffFilter}
      stats={statsMap}
      statsPending={listPending}
    >
      <FileContentArea
        mode={mode}
        onMode={setMode}
        file={activeFile}
        fileMissing={fileMissing}
        canEdit={viewSource.type === 'worktree'}
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
