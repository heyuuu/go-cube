// 轻量自绘弹窗壳（最小 overlay + 卡片 + Escape/点遮罩关闭）：ConfirmDialog、
// worktree 写侧弹窗、workspace 声明弹窗共用的骨架。刻意不引入 shadcn ui/dialog——
// 现有场景都是一张卡片的简单表单/确认，引入门户组件收益不成比例。
import { useEffect, type ReactNode } from 'react';

import { cn } from '@/lib/utils';

export function DialogShell({
  title,
  onClose,
  children,
  className,
}: {
  title: string;
  onClose: () => void;
  children: ReactNode;
  className?: string; // 追加到卡片（如 workspace 弹窗的 max-w-lg + 滚动）
}) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={onClose}>
      <div
        className={cn('mx-4 w-full max-w-sm rounded-lg border border-border bg-background p-4 shadow-lg', className)}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="text-sm font-semibold">{title}</div>
        {children}
      </div>
    </div>
  );
}
