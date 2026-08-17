// Projects 页共用部件：行内打开动作 + tag 徽标。表格行、树项目行、详情抽屉三处使用。
import { ChevronDown } from 'lucide-react';

import type { Opener, Project } from '@/api/client';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import { useOpenerOpen } from '@/queries/project';

import { quickOpens, tagVariants } from './shared';

// 行内打开动作：快捷图标（按已配置 opener 过滤）+ 全量下拉。
export function ProjectActions({
  p,
  openerList,
  open,
  onOpen,
}: {
  p: Project;
  openerList: Opener[];
  open: ReturnType<typeof useOpenerOpen>;
  onOpen: (path: string, app: string) => void;
}) {
  const openerNames = new Set(openerList.map((op) => op.name));
  return (
    <div className="flex items-center gap-0.5" onClick={(e) => e.stopPropagation()}>
      {quickOpens
        .filter((q) => openerNames.has(q.opener))
        .map((q) => (
          <Button
            key={q.opener}
            variant="ghost"
            size="icon-sm"
            title={q.title}
            aria-label={`${q.title}（${p.name}）`}
            disabled={open.isPending && open.variables?.path === p.path && open.variables?.app === q.opener}
            onClick={() => onOpen(p.path, q.opener)}
          >
            {q.icon}
          </Button>
        ))}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label={`打开 ${p.name}`} />}>
          <ChevronDown className="size-3.5" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-32">
          {/* Base UI 的 GroupLabel 必须包在 Group 内，否则运行时抛 MenuGroupContext missing */}
          <DropdownMenuGroup>
            <DropdownMenuLabel>打开方式</DropdownMenuLabel>
            {openerList.map((op) => (
              <DropdownMenuItem
                key={op.name}
                onClick={() => onOpen(p.path, op.name)}
                disabled={open.isPending && open.variables?.path === p.path && open.variables?.app === op.name}
              >
                {op.name}
              </DropdownMenuItem>
            ))}
            {openerList.length === 0 && <div className="px-2 py-1.5 text-xs text-muted-foreground">未配置 opener</div>}
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

// tag 徽标（共用配色）
export function TagBadges({ tags, className }: { tags?: string[] | null; className?: string }) {
  if (!tags || tags.length === 0) return <span className="text-muted-foreground">-</span>;
  return (
    <span className={cn('flex flex-wrap gap-1', className)}>
      {tags.map((t) => (
        <Badge key={t} variant={tagVariants[t] ?? 'outline'}>
          {t}
        </Badge>
      ))}
    </span>
  );
}
