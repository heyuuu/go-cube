// settings 分区共用的表格拖拽排序 hook（Grip 手柄发起、乐观顺序、失效回落）。
// 交互模板源自 Opener 分区（提案 1025 定型），扫描规则 / clone 规则分区复用。
import { useState } from 'react';

import { cn } from '@/lib/utils';

// 冻结列样式：首列贴左、操作贴右，实心 bg 遮住下层滑过的单元格，
// 分隔线用 inset 阴影而非 border（border-collapse 下 border 不随 sticky 单元格移动）；
// 行 hover 靠 group 保持整行联动（sticky 单元格自身的 bg 会盖掉 tr 的 hover bg）
export const STICKY_LEFT =
  'sticky left-0 z-10 bg-background group-hover/row:bg-muted/50 shadow-[inset_-1px_0_0_var(--border)]';
export const STICKY_RIGHT =
  'sticky right-0 z-10 bg-background group-hover/row:bg-muted/50 shadow-[inset_1px_0_0_var(--border)]';

// armed 记录「按住 grip 的行」——只有手柄能发起拖拽（整行 draggable 会干扰按钮点击
// 与文本选择）；order 是乐观顺序，与服务端列表长度不一致（增删后）即失效回落。
// onReorder 直接回传排好序的行，调用方自行映射提交。
export function useDragOrder<T>(keyOf: (item: T) => string, list: T[], onReorder: (rows: T[]) => void) {
  const [order, setOrder] = useState<string[] | null>(null);
  const [armed, setArmed] = useState<string | null>(null);
  const [dragIdx, setDragIdx] = useState<number | null>(null);
  const [overIdx, setOverIdx] = useState<number | null>(null);

  // 个位数列表，重排计算不值得 useMemo（直接算还免去 list 引用不稳的依赖告警）
  const rows = (() => {
    if (order === null || order.length !== list.length) return list;
    const byKey = new Map(list.map((item) => [keyOf(item), item] as const));
    const sorted: T[] = [];
    for (const k of order) {
      const hit = byKey.get(k);
      if (!hit) return list;
      sorted.push(hit);
    }
    return sorted;
  })();

  const dropTo = (target: number) => {
    if (dragIdx === null || dragIdx === target) return;
    const next = [...rows];
    // 插入位语义：从上往下拖插到 target 之后、从下往上拖插到 target 之前（与高亮线一致）
    const insertAt = dragIdx < target ? target + 1 : target;
    const [moved] = next.splice(dragIdx, 1);
    next.splice(insertAt, 0, moved);
    setOrder(next.map(keyOf));
    onReorder(next);
  };

  const reset = () => {
    setArmed(null);
    setDragIdx(null);
    setOverIdx(null);
  };

  const rowProps = (item: T, i: number) => ({
    className: cn(
      'group/row',
      i === dragIdx && 'opacity-40',
      // 插入位高亮：inset 阴影画线，避免 border 变宽引起行高跳动
      dragIdx !== null &&
        i === overIdx &&
        i !== dragIdx &&
        (dragIdx < i ? 'shadow-[inset_0_-2px_0_var(--primary)]' : 'shadow-[inset_0_2px_0_var(--primary)]'),
    ),
    draggable: armed === keyOf(item),
    onDragStart: (e: React.DragEvent) => {
      setDragIdx(i);
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', keyOf(item));
    },
    onDragOver: (e: React.DragEvent) => {
      e.preventDefault();
      setOverIdx(i);
    },
    onDrop: (e: React.DragEvent) => {
      e.preventDefault();
      dropTo(i);
    },
    onDragEnd: reset,
  });

  // grip 手柄属性：按下才 armed（行变为 draggable），松开解除
  const gripProps = (item: T) => ({
    className: 'size-3.5 shrink-0 cursor-grab text-muted-foreground/40',
    onPointerDown: () => setArmed(keyOf(item)),
    onPointerUp: () => setArmed(null),
    'aria-label': '拖动排序',
  });

  return { rows, rowProps, gripProps };
}
