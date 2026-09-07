import type { ReactNode } from 'react';

import { useLocalPref } from '@/hooks/use-local-pref';

import type { TreeSource } from '../params';
import { PanelSplitter } from '../splitter';

import { FileTree, type FileStat } from './file-tree';

// 左侧文件树列的公共封装（code / diff 面板共用）：FileTree + 宽度拖拽 +
// 树形/平摊、全量/差异 视图偏好持久化（localStorage，按 storageKey 隔离）。
// 偏好由面板持有（filter 等派生逻辑要用 scope），FileTreePane 只负责渲染布局。
// 面板差异只体现在包装：stats 来源（changes vs diff 接口）、默认 scope
// （code=全量、diff=差异）、工具条附加区（diff 显示对比模式与数量）。

const MIN_WIDTH = 160;
const MAX_WIDTH = 640;

export function useTreePanePrefs(storageKey: string, defaultScope: 'all' | 'diff') {
  const [view, setView] = useLocalPref<'tree' | 'flat'>(
    `${storageKey}.view`,
    'tree',
    (raw) => (raw === 'flat' ? 'flat' : 'tree'),
  );
  const [scope, setScope] = useLocalPref<'all' | 'diff'>(`${storageKey}.scope`, defaultScope, (raw) =>
    raw === 'all' || raw === 'diff' ? raw : defaultScope,
  );
  const [width, setWidth] = useLocalPref<number>(
    `${storageKey}.width`,
    240,
    (raw) => {
      const v = Number(raw);
      return Number.isFinite(v) && v >= MIN_WIDTH && v <= MAX_WIDTH ? v : 240;
    },
  );
  return { view, setView, scope, setScope, width, setWidth };
}

export type TreePanePrefs = ReturnType<typeof useTreePanePrefs>;

export function FileTreePane({
  prefs,
  above,
  below,
  toolbarExtra,
  ...fileTreeProps
}: {
  prefs: TreePanePrefs;
  above?: ReactNode; // 树上方附加行（diff 面板的路径搜索框）
  below?: ReactNode; // 树下方附加区（内容面板的提交详情，自带拖拽分隔条）
  toolbarExtra?: ReactNode; // 树工具条附加区（diff 面板的对比模式·数量）
} & Omit<Parameters<typeof FileTree>[0], 'viewMode' | 'onViewMode' | 'scope' | 'onScope'>) {
  const { view, setView, scope, setScope, width, setWidth } = prefs;
  return (
    <>
      <div className="flex shrink-0 flex-col border-r border-border" style={{ width }}>
        {above}
        <div className="min-h-0 flex-1">
          <FileTree {...fileTreeProps} viewMode={view} onViewMode={setView} scope={scope} onScope={setScope} />
        </div>
        {below}
      </div>
      <PanelSplitter onDelta={(dx) => setWidth((w) => Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, w + dx)))} />
    </>
  );
}

// 单/双源面板的公共骨架：左侧树列（FileTreePane）+ 右侧内容插槽（FileContentArea）。
// 面板只剩各自包装：数据 hooks 与内容区参数（单文件源、diff 左右源、header 徽标）。
export function SourcePanelShell({
  prefs,
  path,
  treeSource,
  selectedFile,
  onPick,
  diffFilter,
  stats,
  statsPending,
  above,
  below,
  toolbarExtra,
  children,
}: {
  prefs: TreePanePrefs;
  path: string;
  treeSource: TreeSource; // 全量范围浏览的源（双选时传 current 侧源）
  selectedFile: string;
  onPick: (file: string) => void;
  diffFilter: Set<string> | null; // 差异范围的文件集（null = 全量树）
  stats: Map<string, FileStat> | null;
  statsPending: boolean;
  above?: ReactNode;
  below?: ReactNode;
  toolbarExtra?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="flex h-full min-h-0">
      <FileTreePane
        prefs={prefs}
        above={above}
        below={below}
        toolbarExtra={toolbarExtra}
        path={path}
        source={treeSource}
        selectedFile={selectedFile}
        onPick={onPick}
        filter={diffFilter}
        stats={stats}
        statsPending={statsPending}
      />
      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">{children}</div>
    </div>
  );
}
