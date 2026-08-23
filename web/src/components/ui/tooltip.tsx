import { Tooltip as TooltipPrimitive } from '@base-ui/react/tooltip';
import type { ReactElement } from 'react';

import { cn } from '@/lib/utils';

// 轻量 tooltip：全局图标栏等处给纯图标控件补 hover 标签（Base UI 版，非 Radix）
export function HintTip({
  label,
  side = 'right',
  children,
  className,
}: {
  label: string;
  side?: 'top' | 'right' | 'bottom' | 'left';
  children: ReactElement;
  className?: string;
}) {
  return (
    <TooltipPrimitive.Root>
      <TooltipPrimitive.Trigger render={children} />
      <TooltipPrimitive.Portal>
        <TooltipPrimitive.Positioner side={side} sideOffset={6} className="z-50">
          <TooltipPrimitive.Popup
            className={cn(
              'rounded-md bg-popover px-2 py-1 text-xs text-popover-foreground shadow-md ring-1 ring-foreground/10',
              className,
            )}
          >
            {label}
          </TooltipPrimitive.Popup>
        </TooltipPrimitive.Positioner>
      </TooltipPrimitive.Portal>
    </TooltipPrimitive.Root>
  );
}
