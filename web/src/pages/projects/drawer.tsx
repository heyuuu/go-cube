// 项目详情抽屉：点行内项目名滑出。数据直接用列表项（与旧 UI 一致，不单独 fetch info）。
import { useState, type ReactNode } from 'react';

import type { Opener, Project } from '@/api/client';
import { Badge } from '@/components/ui/badge';
import { useCopied } from '@/hooks/use-copied';
import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { prettyPath } from '@/lib/path';
import { formatDateTime } from '@/lib/time';
import { useProjectOpen, useWorkspaceState } from '@/queries/project';

import { ProjectActions, TagBadges } from './actions';
import { WorkspaceDialog } from './workspace-dialog';

function KV({ k, children }: { k: string; children: ReactNode }) {
  return (
    <>
      <dt className="w-20 shrink-0 text-muted-foreground">{k}</dt>
      <dd className="min-w-0 break-words">{children}</dd>
    </>
  );
}

function DrawerSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section>
      <div className="mb-2 text-xs font-medium text-muted-foreground">{title}</div>
      {children}
    </section>
  );
}

export function ProjectDrawer({
  project,
  home,
  openerList,
  open,
  onOpen,
  onClose,
}: {
  project: Project | null;
  home: string;
  openerList: Opener[];
  open: ReturnType<typeof useProjectOpen>;
  onOpen: (path: string, opener: string, dir?: string) => void;
  onClose: () => void;
}) {
  const { copied, copy } = useCopied();
  const [wsEditOpen, setWsEditOpen] = useState(false);
  const wsState = useWorkspaceState(project?.path ?? null);

  const g = project?.gitInfo;

  return (
    <Sheet
      open={project !== null}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      {project && (
        <SheetContent className="w-full gap-0 sm:max-w-[40rem]">
          <SheetHeader>
            <SheetTitle>{project.name}</SheetTitle>
            <SheetDescription className="font-mono" title={project.path}>
              {prettyPath(project.path, home)}
            </SheetDescription>
          </SheetHeader>

          <div className="flex flex-1 flex-col gap-5 overflow-y-auto px-6 pb-6">
            <DrawerSection title="基本信息">
              <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5">
                <KV k="group">{project.group}</KV>
                <KV k="repo">{g?.repoUrl || '-'}</KV>
                <KV k="tags">
                  <TagBadges tags={project.tags} />
                </KV>
              </dl>
            </DrawerSection>

            <DrawerSection title="git 状态">
              {g ? (
                <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5">
                  <KV k="branch">{g.currentBranch || '-'}</KV>
                  <KV k="default">{g.defaultBranch || '-'}</KV>
                  <KV k="ahead/behind">
                    ↑{g.ahead} / ↓{g.behind}
                  </KV>
                  <KV k="dirty">
                    {g.dirty ? <Badge variant="destructive">dirty</Badge> : <Badge variant="secondary">clean</Badge>}
                  </KV>
                  <KV k="采集时间">{formatDateTime(g.collectedAt)}</KV>
                </dl>
              ) : (
                <div className="text-muted-foreground">未采集（等后台 git 缓存刷新）</div>
              )}
            </DrawerSection>

            <DrawerSection title="动作">
              <div className="flex items-center gap-2">
                <ProjectActions p={project} openerList={openerList} open={open} onOpen={onOpen} />
                <Button variant="outline" size="sm" onClick={() => copy(project.path)}>
                  {copied ? '已复制' : '复制路径'}
                </Button>
              </div>
            </DrawerSection>

            <DrawerSection title="workspace 声明（1030）">
              <div className="flex items-center gap-2 text-sm">
                {wsState.data?.declaredSet ? (
                  <Badge variant="default">显式声明</Badge>
                ) : (wsState.data?.effective?.length ?? 0) > 0 ? (
                  <Badge variant="secondary">探测生效</Badge>
                ) : (
                  <span className="text-muted-foreground">无</span>
                )}
                <span className="text-xs text-muted-foreground">{wsState.data?.effective?.length ?? 0} 个成员</span>
                <Button variant="outline" size="sm" className="ml-auto" onClick={() => setWsEditOpen(true)}>
                  编辑
                </Button>
              </div>
              {(wsState.data?.effective?.length ?? 0) > 0 && (
                <div className="mt-2 space-y-1">
                  {wsState.data!.effective!.map((w) => (
                    <div key={w.path} className="flex items-baseline gap-2 text-xs">
                      <span className="font-medium">{w.name}</span>
                      <span className="font-mono text-muted-foreground">{w.path}</span>
                    </div>
                  ))}
                </div>
              )}
            </DrawerSection>

            {(g?.worktrees?.length ?? 0) > 0 && (
              <DrawerSection title={`worktrees（${g!.worktrees!.length}）`}>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>分支</TableHead>
                      <TableHead>路径</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {g!.worktrees!.map((w) => (
                      <TableRow key={w.path}>
                        <TableCell className="px-2 py-1.5 font-medium">{w.branch || w.path.split('/').pop()}</TableCell>
                        <TableCell className="px-2 py-1.5 font-mono text-xs" title={w.path}>
                          {prettyPath(w.path, home)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </DrawerSection>
            )}
          </div>
          {wsEditOpen && <WorkspaceDialog project={project} onClose={() => setWsEditOpen(false)} />}
        </SheetContent>
      )}
    </Sheet>
  );
}
