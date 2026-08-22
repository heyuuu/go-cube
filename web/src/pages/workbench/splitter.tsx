import { useRef } from 'react';

// 拖拽分隔条：pointer 事件（setPointerCapture 保证移出元素仍持续跟踪）。
// 面板槽位之间（index.tsx）与代码阅读面板的文件树右边界共用
export function PanelSplitter({ onDelta }: { onDelta: (dx: number) => void }) {
  const lastX = useRef(0);
  const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    lastX.current = e.clientX;
    e.currentTarget.setPointerCapture(e.pointerId);
  };
  const onPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!(e.buttons & 1)) return;
    const dx = e.clientX - lastX.current;
    lastX.current = e.clientX;
    if (dx !== 0) onDelta(dx);
  };
  return (
    <div
      role="separator"
      aria-orientation="vertical"
      className="w-1 shrink-0 cursor-col-resize bg-border transition-colors hover:bg-primary/50"
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
    />
  );
}
