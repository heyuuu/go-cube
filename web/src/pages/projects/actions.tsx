// Projects 页共用部件：行内打开动作 + tag 徽标。表格行、树项目行、详情抽屉三处使用。
import { Ellipsis, Folder, GitBranch, Layers } from 'lucide-react';

import type { Opener, Project } from '@/api/client';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { CopyPathItem, OpenWithGroup } from '@/components/open-with-menu';
import { renderIcon } from '@/lib/icon';
import { cn } from '@/lib/utils';
import { useIntentDefaultOpener } from '@/queries/opener';
import { useProjectOpen } from '@/queries/project';

import {
  filterTargets,
  projectTargets,
  quickIntents,
  tagVariants,
  type ProjectTarget,
  type TargetKind,
} from './shared';

// 行内打开动作：快捷图标（intent 槽位 × 默认 opener）+ 全量下拉。
// 多目标项目（1032 worktrees / 1030 workspaces）：opener 挂子菜单选目标（Base UI 的
// SubmenuRoot；同弹层内点 item 切内容的做法不可行——item 点击即关菜单）；
// 单目标点击直达，体验不变。快捷位 = quickIntents 解析默认 opener（未配默认的槽位
// 隐藏，不回落），并按目标策略筛（见 shared.tsx）；全量「打开方式」下拉不筛。
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
  const defaultOpenerOf = useIntentDefaultOpener();
  const allTargets = projectTargets(p);
  const multiTarget = allTargets.length > 1;

  const isPending = (name: string) =>
    open.isPending && open.variables?.path === p.path && open.variables?.opener === name;
  const openTarget = (opener: string, dir: string) => onOpen(p.path, opener, dir || undefined);

  const targetMenuItems = (targets: ProjectTarget[], openerName: string) =>
    targets.map((t) => (
      <DropdownMenuItem
        key={t.dir || '/'}
        onClick={() => openTarget(openerName, t.dir)}
        disabled={isPending(openerName)}
      >
        <TargetKindIcon kind={t.kind} />
        {t.label}
      </DropdownMenuItem>
    ));

  const quickButton = (q: (typeof quickIntents)[number]) => {
    const op = defaultOpenerOf(q.intent);
    if (!op) return null; // 未配默认的槽位隐藏，不回落
    // 该快捷位实际可选的目标（策略筛过）；筛剩单目标时点击直达
    const targets = filterTargets(allTargets, q.targets);
    if (targets.length === 1) {
      return (
        <Button
          key={q.intent}
          variant="ghost"
          size="icon-sm"
          title={op.title}
          aria-label={`${op.title}（${p.name}）`}
          disabled={isPending(op.name)}
          onClick={() => openTarget(op.name, targets[0].dir)}
        >
          {renderIcon(op?.icon, null)}
        </Button>
      );
    }
    return (
      <DropdownMenu key={q.intent}>
        <DropdownMenuTrigger
          render={
            <Button
              variant="ghost"
              size="icon-sm"
              title={`${op.title}（选择目标）`}
              aria-label={`${op.title}（${p.name}）`}
              disabled={isPending(op.name)}
            >
              {renderIcon(op?.icon, null)}
            </Button>
          }
        />
        {/* w-auto 覆盖基类的 w-(--anchor-width)：目标名比触发图标宽得多，按内容撑开 */}
        <DropdownMenuContent align="end" className="w-auto min-w-56">
          <DropdownMenuGroup>
            <DropdownMenuLabel>{op.title} · 选择目标</DropdownMenuLabel>
            {targetMenuItems(targets, op.name)}
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    );
  };

  return (
    <div className="flex items-center gap-0.5" onClick={(e) => e.stopPropagation()}>
      {quickIntents.map(quickButton)}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label={`打开 ${p.name}`} />}>
          <Ellipsis className="size-3.5" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-auto min-w-56">
          <CopyPathItem path={p.path} />
          <DropdownMenuSeparator />
          <OpenWithGroup
            openerList={openerList}
            renderItem={(op) =>
              multiTarget ? (
                <DropdownMenuSub key={op.name}>
                  <DropdownMenuSubTrigger disabled={isPending(op.name)}>
                    {renderIcon(op?.icon, null)}
                    {op.title}
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent className="w-auto min-w-56">
                    {targetMenuItems(allTargets, op.name)}
                  </DropdownMenuSubContent>
                </DropdownMenuSub>
              ) : (
                <DropdownMenuItem key={op.name} onClick={() => openTarget(op.name, '')} disabled={isPending(op.name)}>
                  {renderIcon(op?.icon, null)}
                  {op.title}
                </DropdownMenuItem>
              )
            }
          />
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

// 目标条目的身份图标（按 kind 配色，与文字 label 双通道区分）：
// 根目录=目录（前景色）、worktree=紫罗兰（与 worktree 计数徽标同族）、workspace=绿。
// 不复用 opener 图标——同一菜单内每项都一样，无区分度
export function TargetKindIcon({ kind }: { kind: TargetKind }) {
  if (kind === 'workspace') return <Layers className="size-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" />;
  if (kind === 'worktree') return <GitBranch className="size-3.5 shrink-0 text-violet-600 dark:text-violet-400" />;
  return <Folder className="size-3.5 shrink-0 text-muted-foreground" />;
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
      <GitBranch className="size-3" data-icon="inline-start" /> {n}
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

// 展开的目标子行的动作位：按快捷意图槽位给该目标内联直达图标
// （git 槽位只出现在仓库根目标上、workbench 只在主根上，策略见 shared.tsx）。
export function TargetRowActions({
  p,
  target,
  openerList,
  open,
  onOpen,
}: {
  p: Project;
  target: ProjectTarget;
  openerList: Opener[];
  open: ReturnType<typeof useProjectOpen>;
  onOpen: (path: string, opener: string, dir?: string) => void;
}) {
  const defaultOpenerOf = useIntentDefaultOpener();
  const isPending = (name: string) =>
    open.isPending && open.variables?.path === p.path && open.variables?.opener === name;
  const dir = target.dir || undefined; // 子行目标都打开自身目录（主根子行 = 项目根）
  return (
    <div className="flex items-center gap-0.5" onClick={(e) => e.stopPropagation()}>
      {quickIntents
        .filter((q) => filterTargets([target], q.targets).length > 0)
        .map((q) => {
          const op = defaultOpenerOf(q.intent);
          if (!op) return null; // 未配默认的槽位隐藏，不回落
          return (
            <Button
              key={q.intent}
              variant="ghost"
              size="icon-sm"
              title={`${op.title} · ${target.label}`}
              aria-label={`${op.title}（${p.name} ${target.label}）`}
              disabled={isPending(op.name)}
              onClick={() => onOpen(p.path, op.name, dir)}
            >
              {renderIcon(op.icon, null)}
            </Button>
          );
        })}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label={`打开 ${target.label}`} />}>
          <Ellipsis className="size-3.5" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-auto min-w-56">
          <CopyPathItem path={target.dir || p.path} />
          <DropdownMenuSeparator />
          <OpenWithGroup
            openerList={openerList}
            renderItem={(op) => (
              <DropdownMenuItem
                key={op.name}
                disabled={isPending(op.name)}
                onClick={() => onOpen(p.path, op.name, dir)}
              >
                {renderIcon(op?.icon, null)}
                {op.title}
              </DropdownMenuItem>
            )}
          />
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
