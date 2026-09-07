// Projects 页·树视图行（根 = 分组目录、叶 = 项目）：与列表视图同源的单元格复用 cells。
import { ChevronRight, Folder, FolderGit2 } from 'lucide-react';

import type { Opener, Project } from '@/api/client';
import type { IconDecl } from '@/lib/icon';
import { prettyPath } from '@/lib/path';
import { useProjectOpen } from '@/queries/project';
import type { TreeRow } from '@/lib/tree';
import { cn } from '@/lib/utils';

import { ProjectActions, WorktreeCountBadge } from './actions';
import { ClickBadge, ForgeIcon, GitCell, LastUsedTime } from './cells';
import type { GitStatus } from './filters';
import { tagVariants } from './shared';

export function TreeRowView({
  row,
  home,
  openerList,
  open,
  forgeOf,
  onOpen,
  onToggle,
  onFilterGit,
  onFilterTag,
  onDetail,
}: {
  row: TreeRow;
  home: string;
  openerList: Opener[];
  open: ReturnType<typeof useProjectOpen>;
  forgeOf: (p: Project) => { host: string; icon: IconDecl }[];
  onOpen: (path: string, opener: string, dir?: string) => void;
  onToggle: (path: string) => void;
  onFilterGit: (s: GitStatus) => void;
  onFilterTag: (t: string) => void;
  onDetail: (p: Project) => void;
}) {
  const n = row.node;
  const p = n.project;
  return (
    <div
      className={cn(
        'flex items-center gap-1.5 py-1.5 pr-2',
        (row.hasChildren || p) && 'cursor-pointer hover:bg-muted/40',
      )}
      style={{ paddingLeft: row.depth * 20 + 12 }}
      onClick={() => {
        if (p) onDetail(p);
        else if (row.hasChildren) onToggle(n.path);
      }}
    >
      {row.hasChildren ? (
        <ChevronRight
          className={cn('size-3.5 shrink-0 text-muted-foreground transition-transform', row.expanded && 'rotate-90')}
        />
      ) : (
        <span className="w-3.5 shrink-0" />
      )}
      {p ? (
        <FolderGit2 className="size-3.5 shrink-0 text-primary" />
      ) : (
        <Folder className="size-3.5 shrink-0 text-muted-foreground" />
      )}
      {p ? (
        <button
          type="button"
          className="truncate text-left text-xs font-medium hover:underline"
          title={n.path}
          onClick={() => onDetail(p)}
        >
          {row.isRoot ? prettyPath(n.path, home) : n.name}
        </button>
      ) : (
        <span className="truncate text-xs text-muted-foreground" title={n.path}>
          {n.name}
        </span>
      )}
      {p && (
        <>
          <ForgeIcon matches={forgeOf(p)} />
          {(p.tags ?? []).map((t) => (
            <ClickBadge
              key={t}
              variant={tagVariants[t] ?? 'outline'}
              title={`筛选 tag：${t}`}
              onClick={() => onFilterTag(t)}
            >
              {t}
            </ClickBadge>
          ))}
          <WorktreeCountBadge p={p} />
          <GitCell p={p} onFilter={onFilterGit} />
          <div className="ml-auto flex items-center gap-2">
            {p.lastUsedAt && <LastUsedTime iso={p.lastUsedAt} />}
            <ProjectActions p={p} openerList={openerList} open={open} onOpen={onOpen} />
          </div>
        </>
      )}
    </div>
  );
}
