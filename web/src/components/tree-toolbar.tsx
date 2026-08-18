import { ListCollapse, ListTree } from 'lucide-react';

import { Button } from '@/components/ui/button';

// 目录树通用工具条：展开/折叠全部为内置按钮（调用方注入回调——数据侧的
// 展开语义各树不同：md 树是内存集合，工作台树要异步逐层加载）；
// 其余按钮经 extra 注入（如工作台树的「含 ignored」开关）。
// md 页与工作台代码阅读面板共用，样式统一。
export function TreeToolbar({
  onExpandAll,
  onCollapseAll,
  busy = false,
  extra,
}: {
  onExpandAll: () => void;
  onCollapseAll: () => void;
  busy?: boolean; // 异步逐层加载中（展开全部可能较慢）
  extra?: React.ReactNode;
}) {
  return (
    <div className="flex shrink-0 items-center gap-1 border-b border-border px-1 py-1">
      <Button
        variant="ghost"
        size="sm"
        className="h-6 px-2 text-xs"
        disabled={busy}
        onClick={onExpandAll}
        title="展开全部目录"
      >
        <ListTree className="mr-1 size-3.5" />
        展开
      </Button>
      <Button
        variant="ghost"
        size="sm"
        className="h-6 px-2 text-xs"
        onClick={onCollapseAll}
        title="折叠全部目录（保留顶层）"
      >
        <ListCollapse className="mr-1 size-3.5" />
        折叠
      </Button>
      {extra}
    </div>
  );
}
