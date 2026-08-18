import { Box } from 'lucide-react';
import { useSearchParams } from 'react-router';

import { GitTreePanel } from './panels/git-tree-panel';
import { ContentPanelPlaceholder, TerminalPanelPlaceholder } from './panels/placeholders';
import { readWorkbenchParams, writePathParam } from './params';
import { PathEntry } from './path-entry';

// 工作台页面（提案 1010-workbench基座）：以任意本机 git 目录为输入，
// 聚合 git 可视化 / 代码阅读 / diff / PTY。独立于主应用 Layout（同 /md）。
// URL 是面板间唯一总线（当前仅 path；选中态参数由 1011/1012 引入）。
// 布局为固定骨架：左 git 树 + 右内容区 + 底部 PTY 抽屉，自定义布局在 1015。
export function WorkbenchPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const { path } = readWorkbenchParams(searchParams);

  const submitPath = (value: string) => {
    const next = new URLSearchParams(searchParams);
    writePathParam(next, value);
    setSearchParams(next, { replace: true });
  };

  if (!path) {
    return (
      <main className="h-dvh bg-background px-4">
        <PathEntry initial="" onSubmit={submitPath} />
      </main>
    );
  }

  return (
    <div className="flex h-dvh flex-col bg-background text-foreground">
      <header className="flex items-center gap-2 border-b border-border px-3 py-2">
        <Box className="size-4 text-primary" />
        <span className="text-xs font-semibold tracking-wide">工作台</span>
        <span className="truncate text-xs text-muted-foreground">{path}</span>
        <button
          type="button"
          className="ml-auto rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground"
          onClick={() => submitPath('')}
        >
          更换目录
        </button>
      </header>
      <div className="flex min-h-0 flex-1">
        <aside className="w-64 shrink-0 overflow-y-auto border-r border-border">
          <GitTreePanel path={path} />
        </aside>
        <main className="min-w-0 flex-1">
          <ContentPanelPlaceholder hasSource={false} />
        </main>
      </div>
      <TerminalPanelPlaceholder />
    </div>
  );
}
