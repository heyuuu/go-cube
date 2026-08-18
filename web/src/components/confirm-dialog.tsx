import { Button } from '@/components/ui/button';

// 轻量确认弹窗（提案 1012 的「开启编辑/保存/丢弃改动」确认，均为一段文案 + 两按钮场景）。
// 用最小自绘 overlay 而非引入新的 shadcn 组件依赖；如后续需求变复杂再升级成 ui/dialog。
export function ConfirmDialog({
  open,
  title,
  message,
  confirmText = '确认',
  danger = false,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  title: string;
  message: string;
  confirmText?: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={onCancel}>
      <div
        className="mx-4 w-full max-w-sm rounded-lg border border-border bg-background p-4 shadow-lg"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="text-sm font-semibold">{title}</div>
        <div className="mt-2 text-xs leading-relaxed text-muted-foreground">{message}</div>
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="outline" size="sm" onClick={onCancel}>
            取消
          </Button>
          <Button variant={danger ? 'destructive' : 'default'} size="sm" onClick={onConfirm}>
            {confirmText}
          </Button>
        </div>
      </div>
    </div>
  );
}
