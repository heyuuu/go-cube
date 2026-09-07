import { ChevronsDownUp, ChevronsUpDown } from 'lucide-react';

import { Button } from '@/components/ui/button';

// 目录树通用工具条：展开/折叠全部为内置 icon 按钮（功能描述见 title 悬停提示；
// 调用方注入回调——数据侧的展开语义各树不同：md 树是内存集合，工作台树为一次全量拉取）；
// 其余按钮经 extra 注入。md 页与工作台代码阅读面板共用，样式统一。
export function TreeToolbar({
  onExpandAll,
  onCollapseAll,
  leading,
  extra,
}: {
  onExpandAll: () => void;
  onCollapseAll: () => void;
  leading?: React.ReactNode; // 前置按钮（渲染在展开/折叠之前）
  extra?: React.ReactNode;
}) {
  return (
    <div className="flex shrink-0 items-center gap-1 border-b border-border px-1 py-1">
      {leading}
      <Button
        variant="ghost"
        size="icon-sm"
        className="text-muted-foreground"
        onClick={onExpandAll}
        title="展开全部目录"
        aria-label="展开全部目录"
      >
        <ChevronsUpDown className="size-3.5" />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        className="text-muted-foreground"
        onClick={onCollapseAll}
        title="折叠全部目录（保留顶层）"
        aria-label="折叠全部目录（保留顶层）"
      >
        <ChevronsDownUp className="size-3.5" />
      </Button>
      {extra}
    </div>
  );
}
