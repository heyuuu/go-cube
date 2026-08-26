import { ChevronDown, Check } from 'lucide-react';

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useProjectList } from '@/queries/project';
import { cn } from '@/lib/utils';

// 工作台顶栏项目切换（提案 1023）：复用 projects 列表数据源，下拉直达其他项目的工作台，
// 不必退回项目列表再下钻。当前目录不在项目列表时（任意 git 目录入口）仅展示路径，菜单仍可跳。
export function ProjectSwitcher({ current, onSwitch }: { current: string; onSwitch: (path: string) => void }) {
  const projects = useProjectList().data?.list ?? [];

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
          <DropdownMenuLabel>切换项目</DropdownMenuLabel>
        </DropdownMenuGroup>
        {projects.length === 0 && <div className="px-2 py-1.5 text-xs text-muted-foreground">项目列表为空</div>}
        {projects.map((p) => (
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
