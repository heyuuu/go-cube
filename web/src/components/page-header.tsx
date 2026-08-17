import type { ReactNode } from 'react';

// 页头：标题 + 元信息（左）与动作区（右），列表/配置页共用
export function PageHeader({ title, meta, actions }: { title: string; meta?: ReactNode; actions?: ReactNode }) {
  return (
    <header className="flex items-start justify-between px-6 pt-5 pb-3">
      <div className="min-w-0">
        <h1 className="text-lg font-semibold">{title}</h1>
        {meta ? <div className="mt-1 text-xs text-muted-foreground">{meta}</div> : null}
      </div>
      {actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
    </header>
  );
}
