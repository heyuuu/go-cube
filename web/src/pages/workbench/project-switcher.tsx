import { ChevronDown, Check } from 'lucide-react';

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import { useProjectList } from '@/queries/project';

// 工作台顶栏项目切换（提案 1023）：复用 projects 列表数据源，下拉直达其他项目的工作台，
// 不必退回项目列表再下钻。当前目录不在项目列表时（任意 git 目录入口）仅展示路径，菜单仍可跳。
export function ProjectSwitcher({ current, onSwitch }: { current: string; onSwitch: (path: string) => void }) {
  const projects = useProjectList().data?.list ?? [];

  // 只保留最近使用过的 10 个项目：切换是高频直达场景，全量列表留给项目页
  const recents = projects
    .filter((p) => p.lastUsedAt)
    .sort((a, b) => Date.parse(b.lastUsedAt!) - Date.parse(a.lastUsedAt!))
    .slice(0, 10);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className={cn(
          'flex min-w-0 items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground',
          'hover:bg-accent hover:text-accent-foreground',
        )}
        render={<button type="button" title={current} />}
      >
        <span className="truncate font-mono">{current}</span>
        <ChevronDown className="size-3 shrink-0" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-64">
        <DropdownMenuGroup>
          <DropdownMenuLabel>切换项目（最近使用）</DropdownMenuLabel>
        </DropdownMenuGroup>
        {recents.length === 0 && <div className="px-2 py-1.5 text-xs text-muted-foreground">暂无最近使用的项目</div>}
        {recents.map((p) => (
          <DropdownMenuItem key={p.path} onClick={() => p.path !== current && onSwitch(p.path)}>
            <Check className={cn('size-3 shrink-0', p.path !== current && 'invisible')} />
            <span className="truncate">
              {p.group ? `${p.group}/` : ''}
              {p.name}
            </span>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
