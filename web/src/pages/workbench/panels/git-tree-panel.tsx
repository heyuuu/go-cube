import { GitBranch, Monitor } from 'lucide-react';

import { ErrorBanner } from '@/components/error-banner';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { useWorkbenchInfo, useWorkbenchRefs } from '@/queries/workbench';

// git 树面板占位（完整实现在提案 1011）：先展示工作副本列表与分支基础信息，
// 让基座可验证、可用；commit 图 / 状态区 / 单双选交互随 1011 落地。
export function GitTreePanel({ path }: { path: string }) {
  const info = useWorkbenchInfo(path);
  const refs = useWorkbenchRefs(path);

  if (info.isPending || refs.isPending) {
    return <div className="p-3 text-xs text-muted-foreground">加载中…</div>;
  }
  if (info.isError) {
    return <ErrorBanner message={info.error.message} />;
  }

  return (
    <div className="flex h-full flex-col overflow-y-auto">
      <Section title="工作副本" icon={<Monitor className="size-3.5" />}>
        {info.data?.worktrees?.map((wt) => (
          <div key={wt.path} className="flex items-center gap-2 rounded-md px-2 py-1.5 text-xs">
            <span className="truncate font-medium">{wt.path}</span>
            {wt.branch ? <Badge variant="secondary">{wt.branch}</Badge> : null}
            {wt.detached ? <Badge variant="outline">detached</Badge> : null}
            {wt.bare ? <Badge variant="outline">bare</Badge> : null}
          </div>
        ))}
      </Section>
      <Section title="分支" icon={<GitBranch className="size-3.5" />}>
        {refs.data?.locals?.map((b) => (
          <div
            key={b}
            className={cn(
              'rounded-md px-2 py-1 text-xs',
              b === refs.data?.current ? 'bg-primary/10 font-medium text-primary' : 'text-muted-foreground',
            )}
          >
            {b}
            {b === refs.data?.current ? ' *' : ''}
          </div>
        ))}
      </Section>
    </div>
  );
}

function Section({ title, icon, children }: { title: string; icon: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="border-b border-border p-2">
      <div className="flex items-center gap-1.5 px-1 pb-1.5 text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
        {icon}
        {title}
      </div>
      {children}
    </section>
  );
}
