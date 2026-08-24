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

// 内容面板（code + diff 合并）：统一为「current [+ base]」视图模型——
//   current = 当前查看的版本；base 空 = diff 模式与相对基准比
//   （worktree vs HEAD、ref/commit vs 父提交，后端解析）；base 有 = 与 base 比。
// URL 参数 current / base（见 ../params.ts 的角色命名说明）。
// 树列/内容区/编辑流均为公共组件（SourcePanelShell / FileContentArea / useFileEditing）。

// 树偏好键前缀（视图/范围/宽度，见 tree-pane.tsx 的 useTreePanePrefs）
const TREE_PREFS_KEY = 'cube.workbench.contenttree';
// 内容模式（单文件/diff）持久化键
const MODE_KEY = 'cube.workbench.content.mode';

export function ContentViewPanel({ params }: { params: WorkbenchParams }) {
  const { path } = params;
  const [searchParams, setSearchParams] = useSearchParams();
  const file = searchParams.get('file') ?? '';

  const viewSource = params.current;
  const viewBase = params.base;
  // 空源占位（hooks 必须无条件调用；空源走 enabled=false）
  const src = viewSource ?? EMPTY_SOURCE;

  const treePrefs = useTreePanePrefs(TREE_PREFS_KEY, 'all');
  // 内容模式默认随双选态：双选进 diff、单选进单文件；用户手动切换后保留并持久化
  // （刷新恢复），双选↔单选切换时重置（render 期 setState 的派生重置模式）
  const baseKey = viewBase ? `${viewBase.type}:${viewBase.id}` : '';
  const defaultMode = (): ContentMode => {
    const saved = localStorage.getItem(MODE_KEY);
    if (saved === 'diff' || saved === 'file') return saved;
    return viewBase ? 'diff' : 'file';
  };
  const [mode, setMode] = useState<ContentMode>(defaultMode);
  const [prevBaseKey, setPrevBaseKey] = useState(baseKey);
  if (prevBaseKey !== baseKey) {
    setPrevBaseKey(baseKey);
    const next: ContentMode = viewBase ? 'diff' : 'file';
    setMode(next);
    localStorage.setItem(MODE_KEY, next);
  }
  const switchMode = (m: ContentMode) => {
    setMode(m);
    localStorage.setItem(MODE_KEY, m);
  };

  // 与 FileTree 同 key 的树数据（react-query 去重复用）：单选时做 file 回退判定——
  // 切换 commit/分支后 file 参数可能指向新源里不存在的文件。只做显示层回退不改写
  // URL——file 参数指向用户最后的选择，源切换是临时浏览上下文
  const tree = useWorkbenchTree(path, src);
  const treeList = tree.data?.list ?? null;

  // 差异范围的变更清单：base 空 = 相对基准（changes，含 untracked 与行级统计）；
  // base 有 = 两源对比（diff）。两者同构，行级统计后端均已注入
  const changes = useWorkbenchChanges(path, src, !viewBase);
  const diff = useWorkbenchDiff(path, viewBase ?? EMPTY_SOURCE, src); // 空 base 时 enabled=false（base.id 为空）
  const changeList = viewBase ? (diff.data?.list ?? []) : (changes.data?.list ?? []);
  const listPending = viewBase ? diff.isPending : changes.isPending;

  // 回退判定：file 不在当前源中 → 根目录 README.md → 都没有则空并提示。
  // 删除文件不在 ls-files 清单里但可选中（树里由变更集并入），不算 missing。
  // 只做显示层回退不改写 URL——file 参数指向用户最后的选择，源切换是临时浏览上下文
  const deletedPaths = new Set(changeList.filter((e) => e.status === 'deleted').map((e) => e.path));
  const fileMissing = !!treeList && !!file && !treeList.includes(file) && !deletedPaths.has(file);
  const activeFile = !fileMissing ? file : treeList?.includes('README.md') ? 'README.md' : '';

  const content = useWorkbenchFile(path, src, activeFile);
  // rename 条目基准侧路径不同：未改内容的 rename 两侧字节相同，diff 应显示「内容一致」
  // 而非「一侧全文」（左侧按旧路径读）；改了内容则呈现真实的行级差异
  const renameOldPath = changeList.find((e) => e.path === activeFile && e.status === 'renamed')?.oldPath ?? '';
  const fileDiff = useWorkbenchFileDiff(path, viewBase, src, activeFile, mode === 'diff', renameOldPath);

  const statsMap = new Map(
    changeList.map((e) => [
      e.path,
      { adds: e.adds, dels: e.dels, binary: e.binary, status: e.status, oldPath: e.oldPath || undefined },
    ]),
  );
  const editing = useFileEditing(path, src, activeFile, content.data?.content ?? '');

  // 本地路径搜索：空格分隔多个子串，须全部命中（AND）；差异范围对
  // path + rename 旧路径匹配，全量范围对文件路径匹配（treeList 与 FileTree 同 key，无额外请求）
  const [query, setQuery] = useState('');
  const terms = query.trim().split(/\s+/).filter(Boolean);
  const matchAll = (s: string) => terms.every((t) => s.includes(t));
  const entries = changeList.filter((e) => terms.length === 0 || matchAll(e.path + ' ' + (e.oldPath ?? '')));
  const diffFilter =
    treePrefs.scope === 'diff'
      ? new Set(entries.map((e) => e.path))
      : terms.length > 0
        ? new Set((treeList ?? []).filter(matchAll))
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
        <div className="shrink-0 border-b border-border px-2 py-1.5">
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="搜索路径（子串）…"
            className="h-6 text-xs"
          />
        </div>
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
        onMode={switchMode}
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
