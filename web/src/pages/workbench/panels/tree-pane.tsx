import { useEffect, useState, type ReactNode } from 'react';

import { PanelSplitter } from '../splitter';

import { FileTree } from './file-tree';

// 左侧文件树列的公共封装（code / diff 面板共用）：FileTree + 宽度拖拽 +
// 树形/平摊、全量/差异 视图偏好持久化（localStorage，按 storageKey 隔离）。
// 偏好由面板持有（filter 等派生逻辑要用 scope），FileTreePane 只负责渲染布局。
// 面板差异只体现在包装：stats 来源（changes vs diff 接口）、默认 scope
// （code=全量、diff=差异）、工具条附加区（diff 显示对比模式与数量）。

const MIN_WIDTH = 160;
const MAX_WIDTH = 640;

export function useTreePanePrefs(storageKey: string, defaultScope: 'all' | 'diff') {
  const [view, setView] = useState<'tree' | 'flat'>(() =>
    localStorage.getItem(`${storageKey}.view`) === 'flat' ? 'flat' : 'tree',
  );
  const [scope, setScope] = useState<'all' | 'diff'>(() => {
    const saved = localStorage.getItem(`${storageKey}.scope`);
    return saved === 'all' || saved === 'diff' ? saved : defaultScope;
  });
  const [width, setWidth] = useState(() => {
    const v = Number(localStorage.getItem(`${storageKey}.width`));
    return Number.isFinite(v) && v >= MIN_WIDTH && v <= MAX_WIDTH ? v : 240;
  });

  useEffect(() => {
    localStorage.setItem(`${storageKey}.view`, view);
  }, [storageKey, view]);
  useEffect(() => {
    localStorage.setItem(`${storageKey}.scope`, scope);
  }, [storageKey, scope]);
  useEffect(() => {
    localStorage.setItem(`${storageKey}.width`, String(width));
  }, [storageKey, width]);

  return { view, setView, scope, setScope, width, setWidth };
}

export type TreePanePrefs = ReturnType<typeof useTreePanePrefs>;

export function FileTreePane({
  prefs,
  above,
  toolbarExtra,
  ...fileTreeProps
}: {
  prefs: TreePanePrefs;
  above?: ReactNode; // 树上方附加行（diff 面板的路径搜索框）
  toolbarExtra?: ReactNode; // 树工具条附加区（diff 面板的对比模式·数量）
} & Omit<Parameters<typeof FileTree>[0], 'viewMode' | 'onViewMode' | 'scope' | 'onScope' | 'scopePending'>) {
  const { view, setView, scope, setScope, width, setWidth } = prefs;
  return (
    <>
      <div className="flex shrink-0 flex-col border-r border-border" style={{ width }}>
        {above}
        <div className="min-h-0 flex-1">
          <FileTree
            {...fileTreeProps}
            viewMode={view}
            onViewMode={setView}
            scope={scope}
            onScope={setScope}
          />
        </div>
      </div>
      <PanelSplitter onDelta={(dx) => setWidth((w) => Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, w + dx)))} />
    </>
  );
}
