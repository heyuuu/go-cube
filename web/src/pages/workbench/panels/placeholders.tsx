import { TerminalSquare } from 'lucide-react';

// 内容区占位：代码阅读（1012）/ diff（1013）将在此按选中态切换
export function ContentPanelPlaceholder({ hasSource }: { hasSource: boolean }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2 text-center">
      <div className="text-sm font-medium text-muted-foreground">
        {hasSource ? '已选目标，代码阅读 / diff 面板待实现' : '从左侧选择一个目标开始'}
      </div>
      <div className="text-xs text-muted-foreground/70">代码阅读（1012）与 diff（1013）将在此呈现</div>
    </div>
  );
}

// PTY 抽屉占位（完整实现在提案 1014）：默认收起，只留常驻把手提示
export function TerminalPanelPlaceholder() {
  return (
    <div className="flex items-center justify-center border-t border-border bg-muted/30 py-2 text-xs text-muted-foreground">
      <TerminalSquare className="mr-1.5 size-3.5" />
      终端面板（1014）待实现
    </div>
  );
}
