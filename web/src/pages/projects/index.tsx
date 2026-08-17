import { ChevronRight, Folder, FolderGit2, RefreshCw, RotateCcw } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { useSearchParams } from 'react-router';

import type { Opener, Project } from '@/api/client';
import { EmptyState } from '@/components/empty-state';
import { ErrorBanner } from '@/components/error-banner';
import { PageHeader } from '@/components/page-header';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { guessHome, prettyPath } from '@/lib/path';
import { formatDateTime, prettyTime } from '@/lib/time';
import { buildProjectTree, collectExpandablePaths, flattenTree, type TreeRow } from '@/lib/tree';
import { cn } from '@/lib/utils';
import { useOpenProject, useOpenerList, useProjectList } from '@/queries/project';

import { ProjectActions } from './actions';
import { ProjectDrawer } from './drawer';
import { tagVariants } from './shared';

type GitStatus = 'clean' | 'dirty' | 'ahead' | 'behind' | 'none';

const gitFilters: { value: GitStatus | 'all'; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'clean', label: 'clean' },
  { value: 'dirty', label: 'dirty' },
  { value: 'ahead', label: 'ahead' },
  { value: 'behind', label: 'behind' },
  { value: 'none', label: '未采集' },
];

// 谓词式匹配（非互斥分桶）：项目可能同时 dirty + ahead，
// 筛 ahead 应包含所有 ahead > 0 的项目，而不是被 dirty 优先级吞掉。
function matchGitFilter(p: Project, filter: GitStatus | 'all'): boolean {
  const g = p.gitInfo;
  if (filter === 'all') return true;
  if (!g) return filter === 'none';
  switch (filter) {
    case 'none':
      return false;
    case 'dirty':
      return g.dirty;
    case 'ahead':
      return g.ahead > 0;
    case 'behind':
      return g.behind > 0;
    case 'clean':
      return !g.dirty && g.ahead === 0 && g.behind === 0;
  }
}

// 筛选行标签：名称 + 单选/多选标注（与旧页面一致）
function FilterLabel({ label, mode }: { label: string; mode: '单选' | '多选' }) {
  return (
    <span className="w-20 shrink-0 text-muted-foreground">
      {label}
      <span className="ml-1 text-[0.625rem] opacity-70">{mode}</span>
    </span>
  );
}

// 筛选 chip（单选/多选由调用方控制 active）
function Chip({ active, onClick, children }: { active: boolean; onClick: () => void; children: string }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'rounded-full border px-2.5 py-0.5 text-xs transition-colors',
        active
          ? 'border-primary bg-primary text-primary-foreground'
          : 'text-muted-foreground hover:bg-muted hover:text-foreground',
      )}
    >
      {children}
    </button>
  );
}

// 可点击 badge：点击将对应筛选定位到该值（已是唯一选中则取消）。激活状态由上方 chips 呈现，此处只做可点提示
function ClickBadge({
  variant,
  title,
  onClick,
  children,
}: {
  variant?: 'default' | 'secondary' | 'outline' | 'destructive';
  title: string;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <Badge
      variant={variant}
      title={title}
      onClick={(e) => {
        e.stopPropagation();
        onClick();
      }}
      className="cursor-pointer hover:ring-2 hover:ring-ring/40"
    >
      {children}
    </Badge>
  );
}

function GitCell({ p, onFilter }: { p: Project; onFilter: (s: GitStatus) => void }) {
  const g = p.gitInfo;
  if (!g) {
    return (
      <ClickBadge variant="outline" title="筛选 git：未采集" onClick={() => onFilter('none')}>
        未采集
      </ClickBadge>
    );
  }
  return (
    <div className="flex items-center gap-1.5">
      <span className="font-mono text-xs text-muted-foreground">⎇ {g.currentBranch || g.defaultBranch || '-'}</span>
      {g.dirty && (
        <ClickBadge variant="destructive" title="筛选 git：dirty" onClick={() => onFilter('dirty')}>
          dirty
        </ClickBadge>
      )}
      {g.ahead > 0 && (
        <ClickBadge title="筛选 git：ahead" onClick={() => onFilter('ahead')}>
          ↑{g.ahead}
        </ClickBadge>
      )}
      {g.behind > 0 && (
        <ClickBadge variant="outline" title="筛选 git：behind" onClick={() => onFilter('behind')}>
          ↓{g.behind}
        </ClickBadge>
      )}
      {/* clean 是最干净的状态，不展示徽标；筛选仍走上方 chips 的 clean 选项 */}
    </div>
  );
}

// 树行：目录行整行点击折叠/展开；项目行带 tags / git 信息与打开动作（根行显示 ~ 缩写路径）
function TreeRowView({
  row,
  home,
  openerList,
  open,
  onOpen,
  onToggle,
  onFilterGit,
  onFilterTag,
  onDetail,
}: {
  row: TreeRow;
  home: string;
  openerList: Opener[];
  open: ReturnType<typeof useOpenProject>;
  onOpen: (path: string, app: string) => void;
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
          <GitCell p={p} onFilter={onFilterGit} />
          <div className="ml-auto">
            <ProjectActions p={p} openerList={openerList} open={open} onOpen={onOpen} />
          </div>
        </>
      )}
    </div>
  );
}

export function ProjectsPage() {
  const list = useProjectList();
  const openers = useOpenerList();
  const open = useOpenProject();

  const [keyword, setKeyword] = useState('');
  const [groupFilter, setGroupFilter] = useState<string[]>([]);
  const [gitFilter, setGitFilter] = useState<GitStatus | 'all'>('all');
  const [tagFilter, setTagFilter] = useState('all');
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [openError, setOpenError] = useState('');
  const [drawer, setDrawer] = useState<Project | null>(null);
  // 显示模式走 URL（?view=tree）：可刷新保状态、可分享；两种模式共用筛选状态，切换不丢
  const [searchParams, setSearchParams] = useSearchParams();
  const mode: 'table' | 'tree' = searchParams.get('view') === 'tree' ? 'tree' : 'table';
  const [treeExpanded, setTreeExpanded] = useState<ReadonlySet<string>>(new Set());

  const projects = list.data?.list ?? [];
  const home = guessHome(projects.map((p) => p.path));
  const groups = [...new Set(projects.map((p) => p.group))].sort();
  const tags = [...new Set(projects.flatMap((p) => p.tags ?? []))].sort();

  const filtered = projects.filter((p) => {
    const kw = keyword.trim().toLowerCase();
    if (kw && !(p.name.toLowerCase().includes(kw) || p.path.toLowerCase().includes(kw))) return false;
    if (groupFilter.length > 0 && !groupFilter.includes(p.group)) return false;
    if (!matchGitFilter(p, gitFilter)) return false;
    if (tagFilter !== 'all' && !(p.tags ?? []).includes(tagFilter)) return false;
    return true;
  });

  function toggleGroup(g: string) {
    setGroupFilter((prev) => (prev.includes(g) ? prev.filter((x) => x !== g) : [...prev, g]));
  }

  // badge 点击筛选：定位到唯一值；再点一次（已是唯一选中）则取消
  function toggleGroupSolo(g: string) {
    setGroupFilter((prev) => (prev.length === 1 && prev[0] === g ? [] : [g]));
  }

  function toggleTagSolo(t: string) {
    setTagFilter((prev) => (prev === t ? 'all' : t));
  }

  function toggleGitSolo(s: GitStatus) {
    setGitFilter((prev) => (prev === s ? 'all' : s));
  }

  function resetFilters() {
    setKeyword('');
    setGroupFilter([]);
    setGitFilter('all');
    setTagFilter('all');
  }

  function toggleSelect(path: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  }

  function switchMode(next: 'table' | 'tree') {
    setSearchParams(next === 'tree' ? { view: 'tree' } : {});
  }

  function toggleTreeNode(path: string) {
    setTreeExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  }

  function openProject(path: string, app: string) {
    setOpenError('');
    open.mutate({ path, app }, { onError: (e) => setOpenError(`打开失败：${e.message}`) });
  }

  const error = list.error ? `加载失败：${list.error.message}` : openError || '';
  const openerList = openers.data?.list ?? [];

  // 树模式：从当前筛选结果前端构建（根恒展开，其余按 treeExpanded）
  const treeRoot = mode === 'tree' ? buildProjectTree(filtered) : null;
  const treeRows = treeRoot ? flattenTree(treeRoot, (path) => treeExpanded.has(path)) : [];

  function expandAllTree() {
    if (treeRoot) setTreeExpanded(new Set(collectExpandablePaths(treeRoot)));
  }

  function collapseAllTree() {
    setTreeExpanded(new Set());
  }

  return (
    <div>
      <PageHeader
        title="Projects"
        meta={
          <div>
            项目列表：
            <span title={formatDateTime(list.data?.scanUpdatedAt)}>{prettyTime(list.data?.scanUpdatedAt)}</span>
            {' ｜ '}git 状态：
            <span title={formatDateTime(list.data?.gitUpdatedAt)}>{prettyTime(list.data?.gitUpdatedAt)}</span>
          </div>
        }
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={resetFilters} title="清空搜索与筛选条件">
              <RotateCcw data-icon="inline-start" />
              重置
            </Button>
            <Button variant="outline" size="sm" onClick={() => list.refetch()} disabled={list.isFetching}>
              <RefreshCw className={cn(list.isFetching && 'animate-spin')} data-icon="inline-start" />
              {list.isFetching ? '刷新中…' : '刷新'}
            </Button>
          </div>
        }
      />

      {error && <ErrorBanner message={error} />}

      {/* 工具条：搜索 + 计数 */}
      <div className="flex items-center gap-3 px-6 pb-2">
        <Input
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          placeholder="搜索项目名 / 路径"
          className="w-64"
        />
        <span className="text-xs text-muted-foreground">
          共 <strong className="text-foreground">{filtered.length}</strong> 个
        </span>
        {selected.size > 0 && (
          <span className="text-xs text-muted-foreground">
            已选 <strong className="text-foreground">{selected.size}</strong> 个
            <button type="button" className="ml-2 text-primary hover:underline" onClick={() => setSelected(new Set())}>
              取消
            </button>
          </span>
        )}
        {/* 显示模式切换：列表 / 树 */}
        <div className="ml-auto flex items-center gap-0.5 rounded-md border p-0.5">
          <button
            type="button"
            onClick={() => switchMode('table')}
            className={cn(
              'rounded-sm px-2.5 py-1 text-xs transition-colors',
              mode === 'table' ? 'bg-background font-medium shadow-sm' : 'text-muted-foreground hover:text-foreground',
            )}
          >
            列表
          </button>
          <button
            type="button"
            onClick={() => switchMode('tree')}
            className={cn(
              'rounded-sm px-2.5 py-1 text-xs transition-colors',
              mode === 'tree' ? 'bg-background font-medium shadow-sm' : 'text-muted-foreground hover:text-foreground',
            )}
          >
            树
          </button>
        </div>
      </div>

      {/* 筛选 chips */}
      <div className="flex flex-col gap-1.5 px-6 pb-3 text-xs">
        <div className="flex flex-wrap items-center gap-1.5">
          <FilterLabel label="group" mode="多选" />
          <Chip active={groupFilter.length === 0} onClick={() => setGroupFilter([])}>
            全部
          </Chip>
          {groups.map((g) => (
            <Chip key={g} active={groupFilter.includes(g)} onClick={() => toggleGroup(g)}>
              {g}
            </Chip>
          ))}
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <FilterLabel label="git" mode="单选" />
          {gitFilters.map((s) => (
            <Chip key={s.value} active={gitFilter === s.value} onClick={() => setGitFilter(s.value)}>
              {s.label}
            </Chip>
          ))}
        </div>
        {tags.length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5">
            <FilterLabel label="tag" mode="单选" />
            <Chip active={tagFilter === 'all'} onClick={() => setTagFilter('all')}>
              全部
            </Chip>
            {tags.map((t) => (
              <Chip key={t} active={tagFilter === t} onClick={() => setTagFilter(t)}>
                {t}
              </Chip>
            ))}
          </div>
        )}
      </div>

      {/* 目录树（tree 模式）：直接消费 filtered，筛选即时生效 */}
      {mode === 'tree' ? (
        <div className="px-6 pb-6">
          <div className="flex items-center justify-between pb-2">
            <span className="text-xs text-muted-foreground">
              基于当前筛选结果（<strong className="text-foreground">{filtered.length}</strong> 个）的目录树
            </span>
            <div className="flex gap-1">
              <Button variant="ghost" size="sm" onClick={expandAllTree}>
                全展开
              </Button>
              <Button variant="ghost" size="sm" onClick={collapseAllTree}>
                全折叠
              </Button>
            </div>
          </div>
          {treeRows.length > 0 ? (
            <div className="divide-y rounded-lg border">
              {treeRows.map((row) => (
                <TreeRowView
                  key={row.node.path}
                  row={row}
                  home={home}
                  openerList={openerList}
                  open={open}
                  onOpen={openProject}
                  onToggle={toggleTreeNode}
                  onFilterGit={toggleGitSolo}
                  onFilterTag={toggleTagSolo}
                  onDetail={setDrawer}
                />
              ))}
            </div>
          ) : (
            <div className="rounded-lg border">
              {list.isPending && projects.length === 0 && <EmptyState title="加载中…" />}
              {!list.isPending && !list.error && (
                <EmptyState title="无项目可展示" sub="调整筛选条件或检查 scan 配置。" />
              )}
            </div>
          )}
        </div>
      ) : (
        <div className="px-6 pb-6">
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-9" />
                  <TableHead>name</TableHead>
                  <TableHead className="w-24">group</TableHead>
                  <TableHead className="w-64">git</TableHead>
                  <TableHead className="w-32" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.map((p) => (
                  <TableRow
                    key={p.path}
                    className={cn('cursor-pointer', selected.has(p.path) && 'bg-muted/50')}
                    onClick={() => setDrawer(p)}
                  >
                    <TableCell onClick={(e) => e.stopPropagation()}>
                      <Checkbox
                        checked={selected.has(p.path)}
                        onCheckedChange={() => toggleSelect(p.path)}
                        aria-label={`选择 ${p.name}`}
                      />
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap items-center gap-1.5">
                        <button
                          type="button"
                          className="text-left text-xs font-medium hover:underline"
                          onClick={() => setDrawer(p)}
                        >
                          {p.name}
                        </button>
                        {(p.tags ?? []).map((t) => (
                          <ClickBadge
                            key={t}
                            variant={tagVariants[t] ?? 'outline'}
                            title={`筛选 tag：${t}`}
                            onClick={() => toggleTagSolo(t)}
                          >
                            {t}
                          </ClickBadge>
                        ))}
                      </div>
                      <div className="mt-0.5 font-mono text-xs text-muted-foreground" title={p.path}>
                        {prettyPath(p.path, home)}
                      </div>
                    </TableCell>
                    <TableCell>
                      <ClickBadge
                        variant="secondary"
                        title={`筛选 group：${p.group}`}
                        onClick={() => toggleGroupSolo(p.group)}
                      >
                        {p.group}
                      </ClickBadge>
                    </TableCell>
                    <TableCell>
                      <GitCell p={p} onFilter={toggleGitSolo} />
                    </TableCell>
                    <TableCell onClick={(e) => e.stopPropagation()}>
                      <div className="flex justify-end">
                        <ProjectActions p={p} openerList={openerList} open={open} onOpen={openProject} />
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>

            {list.isPending && projects.length === 0 && <EmptyState title="加载中…" />}
            {!list.isPending && !list.error && filtered.length === 0 && (
              <EmptyState
                title="无匹配项目"
                sub={projects.length === 0 ? '尚未扫描到任何项目，检查 scan 配置。' : '试试调整搜索或筛选条件。'}
              />
            )}
          </div>
        </div>
      )}

      <ProjectDrawer
        project={drawer}
        home={home}
        openerList={openerList}
        open={open}
        onOpen={openProject}
        onClose={() => setDrawer(null)}
      />
    </div>
  );
}
