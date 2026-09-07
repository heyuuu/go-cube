// 列表页共用的筛选 chips 行与可排序表头（Projects / Forges 页共用）。
// FilterRow：一行 = 固定宽度标签（标注单选/多选，让各行 chips 起点对齐）+ chips 容器。
// Chip：圆形筛选 chip（单选/多选语义由调用方控制 active）。
// SortHead：可点击表头，state='asc'|'desc'|null 三态图标，点击触发 onCycle（循环语义归调用方）。
import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react';
import type { ReactNode } from 'react';

import { TableHead } from '@/components/ui/table';
import { cn } from '@/lib/utils';

export function FilterRow({
  label,
  mode,
  children,
  className,
}: {
  label: string;
  mode: '单选' | '多选';
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('flex flex-wrap items-center gap-1.5', className)}>
      <span className="flex w-24 shrink-0 items-baseline gap-1 text-muted-foreground">
        <span className="text-[0.625rem] opacity-70">{mode}</span>
        {label}
      </span>
      {children}
    </div>
  );
}

export function Chip({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'rounded-full border px-2.5 py-0.5 text-xs transition-colors',
        active
          ? 'border-primary bg-primary text-primary-foreground'
          : 'text-muted-foreground hover:bg-muted hover:text-foreground',
      )}
    >
      {children}
    </button>
  );
}

export function SortHead({
  label,
  state,
  onCycle,
  className,
}: {
  label: string;
  state: 'asc' | 'desc' | null;
  onCycle: () => void;
  className?: string;
}) {
  const Icon = !state ? ArrowUpDown : state === 'desc' ? ArrowDown : ArrowUp;
  return (
    <TableHead className={className}>
      <button
        type="button"
        className={cn(
          'flex items-center gap-1 hover:text-foreground',
          state ? 'font-medium text-foreground' : 'text-muted-foreground',
        )}
        onClick={onCycle}
      >
        {label}
        {/* 激活态：主题色箭头 + 圆形底（类似选中态），与未激活的灰色双向箭头一眼区分。
            圆 20px / 图标 12px——留足四周 padding，视觉上箭头才在圆心 */}
        {state ? (
          <span className="flex size-5 items-center justify-center rounded-full bg-primary/15">
            <Icon className="size-3 text-primary" />
          </span>
        ) : (
          <Icon className="size-3" />
        )}
      </button>
    </TableHead>
  );
}
