// Projects 页共用部件：行内打开动作 + tag 徽标。表格行、树项目行、详情抽屉三处使用。
import { ChevronDown, SquareTerminal } from 'lucide-react';
import { useNavigate } from 'react-router';

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
import { renderOpenerIcon } from '@/lib/opener-icon';
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
  onOpen: (path: string, opener: string) => void;
}) {
  const openerByName = new Map(openerList.map((op) => [op.name, op]));
  const openerNames = new Set(openerList.map((op) => op.name));
  const navigate = useNavigate();
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
            disabled={open.isPending && open.variables?.path === p.path && open.variables?.opener === q.opener}
            onClick={() => onOpen(p.path, q.opener)}
          >
            {renderOpenerIcon(openerByName.get(q.opener), q.fallbackIcon)}
          </Button>
        ))}
      {/* 工作台入口（非 opener）：应用内路由跳转，与快捷图标平齐 */}
      <Button
        variant="ghost"
        size="icon-sm"
        title="在工作台打开"
        aria-label={`在工作台打开（${p.name}）`}
        onClick={() => navigate(`/workbench?path=${encodeURIComponent(p.path)}`)}
      >
        <SquareTerminal className="size-3.5" />
      </Button>
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
                disabled={open.isPending && open.variables?.path === p.path && open.variables?.opener === op.name}
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
