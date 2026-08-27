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
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { renderIcon } from '@/lib/icon';
import { cn } from '@/lib/utils';
import { useProjectOpen } from '@/queries/project';

import { projectTargets, quickOpens, tagVariants } from './shared';

// 行内打开动作：快捷图标（按已配置 opener 过滤）+ 全量下拉。
// 多目标项目（根目录 + worktrees，1032）：opener 挂子菜单选目标（Base UI 的
// SubmenuRoot；同弹层内点 item 切内容的做法不可行——item 点击即关菜单）；
// 单目标项目点击直达，体验不变。
export function ProjectActions({
  p,
  openerList,
  open,
  onOpen,
}: {
  p: Project;
  openerList: Opener[];
  open: ReturnType<typeof useProjectOpen>;
  onOpen: (path: string, opener: string, dir?: string) => void;
}) {
  const openerByName = new Map(openerList.map((op) => [op.name, op]));
  const openerNames = new Set(openerList.map((op) => op.name));
  const targets = projectTargets(p);
  const multiTarget = targets.length > 1;

  const isPending = (name: string) =>
    open.isPending && open.variables?.path === p.path && open.variables?.opener === name;
  const openTarget = (opener: string, dir: string) => onOpen(p.path, opener, dir || undefined);

  const targetMenuItems = (openerName: string) =>
    targets.map((t) => (
      <DropdownMenuItem
        key={t.dir || '/'}
        onClick={() => openTarget(openerName, t.dir)}
        disabled={isPending(openerName)}
      >
        {renderIcon(openerByName.get(openerName)?.icon, undefined)}
        {t.label}
      </DropdownMenuItem>
    ));

  const quickButton = (name: string) => {
    const op = openerByName.get(name);
    if (!op) return null;
    const button = (
      <Button
        variant="ghost"
        size="icon-sm"
        title={multiTarget ? `${op.title}（选择目标）` : op.title}
        aria-label={`${op.title}（${p.name}）`}
        disabled={isPending(name)}
      >
        {renderIcon(op?.icon, null)}
      </Button>
    );
    if (!multiTarget) {
      return (
        <Button
          key={name}
          variant="ghost"
          size="icon-sm"
          title={op.title}
          aria-label={`${op.title}（${p.name}）`}
          disabled={isPending(name)}
          onClick={() => openTarget(name, '')}
        >
          {renderIcon(op?.icon, null)}
        </Button>
      );
    }
    return (
      <DropdownMenu key={name}>
        <DropdownMenuTrigger render={button} />
        <DropdownMenuContent align="end" className="min-w-48">
          <DropdownMenuGroup>
            <DropdownMenuLabel>{op.title} · 选择目标</DropdownMenuLabel>
            {targetMenuItems(name)}
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    );
  };

  return (
    <div className="flex items-center gap-0.5" onClick={(e) => e.stopPropagation()}>
      {quickOpens.filter((name) => openerNames.has(name)).map(quickButton)}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label={`打开 ${p.name}`} />}>
          <ChevronDown className="size-3.5" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-32">
          {/* Base UI 的 GroupLabel 必须包在 Group 内，否则运行时抛 MenuGroupContext missing */}
          <DropdownMenuGroup>
            <DropdownMenuLabel>打开方式</DropdownMenuLabel>
            {openerList.map((op) =>
              multiTarget ? (
                <DropdownMenuSub key={op.name}>
                  <DropdownMenuSubTrigger disabled={isPending(op.name)}>
                    {renderIcon(op?.icon, null)}
                    {op.title}
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent>{targetMenuItems(op.name)}</DropdownMenuSubContent>
                </DropdownMenuSub>
              ) : (
                <DropdownMenuItem key={op.name} onClick={() => openTarget(op.name, '')} disabled={isPending(op.name)}>
                  {renderIcon(op?.icon, null)}
                  {op.title}
                </DropdownMenuItem>
              ),
            )}
            {openerList.length === 0 && <div className="px-2 py-1.5 text-xs text-muted-foreground">未配置 opener</div>}
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

// worktree 计数徽标（多目标项目的行内提示，1032）。
// 紫罗兰专属配色：与 tag 徽标（default/outline）和 git 状态徽标视觉区分
export function WorktreeCountBadge({ p }: { p: Project }) {
  const n = p.gitInfo?.worktrees?.length ?? 0;
  if (n === 0) return null;
  return (
    <Badge
      variant="outline"
      title={`${n} 个 worktree（打开时可选目标）`}
      className="border-violet-500/40 bg-violet-500/10 text-violet-600 dark:bg-violet-500/15 dark:text-violet-400"
    >
      ⎇ {n}
    </Badge>
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
