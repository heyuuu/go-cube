import { Box } from 'lucide-react';
import { useSearchParams } from 'react-router';

import { CodeViewPanel } from './panels/code-view-panel';
import { DiffViewPanel } from './panels/diff-view-panel';
import { GitTreePanel } from './panels/git-tree-panel';
import { ContentPanelPlaceholder } from './panels/placeholders';
import { TerminalPanel } from './panels/terminal-panel';
import { readWorkbenchParams, writePathParam } from './params';
import { PathEntry } from './path-entry';

// 工作台页面（提案 1010 基座 + 1011 选择交互）：以任意本机 git 目录为输入，
// 聚合 git 可视化 / 代码阅读 / diff / PTY。独立于主应用 Layout（同 /md）。
// URL 是面板间唯一总线（path + 选中态 source/left/right，见 params.ts）。
// 布局为固定骨架：左 git 树 + 右内容区 + 底部 PTY 抽屉，自定义布局在 1015。
export function WorkbenchPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const params = readWorkbenchParams(searchParams);

  const submitPath = (value: string) => {
    const next = new URLSearchParams(searchParams);
    if (value) {
      writePathParam(next, value);
    } else {
      next.delete('path');
    }
    setSearchParams(next, { replace: true });
  };

  if (!params.path) {
    return (
      <main className="h-dvh bg-background px-4">
        <PathEntry initial="" onSubmit={submitPath} />
      </main>
    );
  }

  const diffMode = params.left && params.right;

  return (
    <div className="flex h-dvh flex-col bg-background text-foreground">
      <header className="flex items-center gap-2 border-b border-border px-3 py-2">
        <Box className="size-4 text-primary" />
        <span className="text-xs font-semibold tracking-wide">工作台</span>
        <span className="truncate text-xs text-muted-foreground">{params.path}</span>
        <button
          type="button"
          className="ml-auto rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground"
          onClick={() => submitPath('')}
        >
          更换目录
        </button>
      </header>
      <div className="flex min-h-0 flex-1">
        <aside className="w-72 shrink-0 overflow-hidden border-r border-border">
          <GitTreePanel params={params} />
        </aside>
        <main className="min-w-0 flex-1">
          {diffMode ? (
            <DiffViewPanel params={params} />
          ) : params.source ? (
            <CodeViewPanel params={params} />
          ) : (
            <ContentPanelPlaceholder title="从左侧选择一个目标开始" />
          )}
        </main>
      </div>
      <TerminalPanel path={params.path} />
    </div>
  );
}
