import {
  ArrowDownWideNarrow,
  ChevronRight,
  GitBranch,
  Layers,
  RefreshCw,
  RotateCcw,
} from 'lucide-react';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';

import type { Project } from '@/api/client';
import { EmptyState } from '@/components/empty-state';
import { ErrorBanner } from '@/components/error-banner';
import { Chip, CycleSortHead, FilterRow } from '@/components/filter-chips';
import { usePersistentSet } from '@/hooks/use-local-pref';
import { PageHeader } from '@/components/page-header';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { matchForgeFilter, repoHostsOf } from '@/lib/forge';
import type { IconDecl } from '@/lib/icon';
import { renderIcon } from '@/lib/icon';
import { guessHome, prettyPath } from '@/lib/path';
import { formatDateTime, prettyTime } from '@/lib/time';
import { buildProjectTree, collectExpandablePaths, flattenTree } from '@/lib/tree';
import { cn } from '@/lib/utils';
import { useForges } from '@/queries/forge';
import { useOpenerList } from '@/queries/opener';
import { useProjectOpen, useProjectList } from '@/queries/project';
import { useScanRules } from '@/queries/scan-rule';

import { ProjectActions, TargetKindIcon, TargetRowActions, WorktreeCountBadge } from './actions';
import { ClickBadge, ForgeIcon, GitCell, LastUsedTime } from './cells';
import { ProjectDrawer } from './drawer';
import {
  countWorkspaces,
  gitFilters,
  isSortMode,
  matchGitFilter,
  sortKeys,
  timeOf,
  type GitStatus,
  type SortKey,
  type SortMode,
} from './filters';
import { projectTargets, tagVariants } from './shared';
import { TreeRowView } from './tree-view';

export function ProjectsPage() {
  const list = useProjectList();
  const openers = useOpenerList();
  const open = useProjectOpen();
  const scanRules = useScanRules();
  const forges = useForges();

  const [openError, setOpenError] = useState('');
  const [drawer, setDrawer] = useState<Project | null>(null);
  // 筛选状态全部走 URL（?q=&group=&git=&tag=，?view= 同理）：刷新/前进后退/跨页往返均无损
  const [searchParams, setSearchParams] = useSearchParams();

  // 增量更新 query 参数（replace 避免每点一个筛选压一条历史）；空值参数不落 URL
  function updateParams(patch: Record<string, string | null>) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        for (const [key, value] of Object.entries(patch)) {
          if (value) next.set(key, value);
          else next.delete(key);
        }
        return next;
      },
      { replace: true },
    );
  }

  const keyword = searchParams.get('q') ?? '';
  const groupFilter = searchParams.get('group')?.split(',').filter(Boolean) ?? [];
  const gitParam = searchParams.get('git');
  const gitFilter: GitStatus | 'all' = gitFilters.some((f) => f.value === gitParam)
    ? (gitParam as GitStatus | 'all')
    : 'all';
  const tagFilter = searchParams.get('tag') ?? 'all';
  const wtFilter = searchParams.get('wt') === '1';
  const wsFilter = searchParams.get('ws') === '1';
  const forgeList = forges.data?.list ?? [];
  // forge 筛选单选值：all / none / other / 已配置 forge host；非法值回落 all
  const forgeParam = searchParams.get('forge');
  const forgeFilter =
    forgeParam && (forgeParam === 'none' || forgeParam === 'other' || forgeList.some((f) => f.host === forgeParam))
      ? forgeParam
      : 'all';

  // 搜索输入本地 state + 300ms debounce 后投影到 URL；URL 侧变化（后退/重置）回灌输入
  const [keywordInput, setKeywordInput] = useState(keyword);
  useEffect(() => {
    const timer = setTimeout(() => {
      if (keywordInput !== keyword) updateParams({ q: keywordInput });
    }, 300);
    return () => clearTimeout(timer);
  }, [keywordInput, keyword]);
  useEffect(() => {
    setKeywordInput(keyword);
  }, [keyword]);

  const mode: 'table' | 'tree' = searchParams.get('view') === 'tree' ? 'tree' : 'table';
  const [treeExpanded, setTreeExpanded] = useState<ReadonlySet<string>>(new Set());

  // 多目标项目展开的目标子行（localStorage 持久化，key 收敛项目路径）
  const targetExpandedPrefs = usePersistentSet('cube.projects.expanded');
  const targetExpanded = targetExpandedPrefs.set;
  const toggleTargetExpanded = targetExpandedPrefs.toggle;

  const projects = list.data?.list ?? [];
  const home = guessHome(projects.map((p) => p.path));
  // group 不排序：Set 去重保留首次出现序 = 项目列表序 = scanRules 规则序（settings 可拖拽调整）
  const groups = [...new Set(projects.map((p) => p.group))];
  // group → icon 映射（来自扫描规则的可选配置；规则未配 icon 的 group 不进 map）
  const groupIcons = new Map<string, IconDecl>(
    (scanRules.data?.list ?? [])
      .filter((r) => r.icon?.value)
      .map((r) => [r.group, { type: r.icon!.type, value: r.icon!.value }]),
  );
  // host → forge icon 映射 + 项目匹配器（repo URL 解析 host 后查表；1040）
  const forgeIcons = new Map<string, IconDecl>(
    (forges.data?.list ?? [])
      .filter((f) => f.icon?.value)
      .map((f) => [f.host, { type: f.icon!.type, value: f.icon!.value }]),
  );
  // 多 remote 项目可命中多个 forge——全部返回，逐个渲染 icon（1042 修）
  const forgeOf = (p: Project) =>
    repoHostsOf(p.gitInfo)
      .map((host) => ({ host, icon: forgeIcons.get(host) }))
      .filter((m): m is { host: string; icon: IconDecl } => Boolean(m.icon));
  const tags = [...new Set(projects.flatMap((p) => p.tags ?? []))].sort();

  const filtered = projects.filter((p) => {
    const kw = keyword.trim().toLowerCase();
    if (kw && !(p.name.toLowerCase().includes(kw) || p.path.toLowerCase().includes(kw))) return false;
    if (groupFilter.length > 0 && !groupFilter.includes(p.group)) return false;
    if (!matchGitFilter(p, gitFilter)) return false;
    if (tagFilter !== 'all' && !(p.tags ?? []).includes(tagFilter)) return false;
    if (wtFilter && (p.gitInfo?.worktrees?.length ?? 0) === 0) return false;
    if (wsFilter && countWorkspaces(p) === 0) return false;
    if (
      !matchForgeFilter(
        repoHostsOf(p.gitInfo),
        forgeFilter,
        forgeList.map((f) => f.host),
      )
    )
      return false;
    return true;
  });

  // 排序是视图偏好（后端返回原始扫描序 + lastUsedAt）：先筛选后排序，与筛选同侧。
  // 默认最近使用（进入页面即有置顶信号），用户点表头/下拉可切换并持久在 URL
  const sortParam = searchParams.get('sort');
  const sortMode: SortMode = isSortMode(sortParam) ? sortParam : 'recent';
  const sorted = [...filtered];
  const key = sortMode.replace(/-desc$/, '') as SortKey | '';
  const desc = sortMode.endsWith('-desc');
  if (key === 'recent') {
    // 未用过的（无 lastUsedAt）靠稳定排序保持原序垫底
    sorted.sort((a, b) =>
      desc ? timeOf(a.lastUsedAt) - timeOf(b.lastUsedAt) : timeOf(b.lastUsedAt) - timeOf(a.lastUsedAt),
    );
  } else if (key === 'name') {
    sorted.sort((a, b) => (desc ? b.name.localeCompare(a.name) : a.name.localeCompare(b.name)));
  } else if (key === 'group') {
    sorted.sort((a, b) => (desc ? b.group.localeCompare(a.group) : a.group.localeCompare(b.group)));
  }

  function toggleGroup(g: string) {
    updateParams({
      group: groupFilter.includes(g)
        ? groupFilter.filter((x) => x !== g).join(',') || null
        : [...groupFilter, g].join(','),
    });
  }

  // badge 点击筛选：定位到唯一值；再点一次（已是唯一选中）则取消
  function toggleGroupSolo(g: string) {
    updateParams({ group: groupFilter.length === 1 && groupFilter[0] === g ? null : g });
  }

  function toggleTagSolo(t: string) {
    updateParams({ tag: tagFilter === t ? null : t });
  }

  function toggleGitSolo(s: GitStatus) {
    updateParams({ git: gitFilter === s ? null : s });
  }

  function setGitFilterValue(v: GitStatus | 'all') {
    updateParams({ git: v === 'all' ? null : v });
  }

  function setWtFilter(on: boolean) {
    updateParams({ wt: on ? '1' : null });
  }

  function setWsFilter(on: boolean) {
    updateParams({ ws: on ? '1' : null });
  }

  function setForgeFilterValue(v: string) {
    updateParams({ forge: v === 'all' ? null : v });
  }

  function setTagFilterValue(t: string) {
    updateParams({ tag: t === 'all' ? null : t });
  }

  function resetFilters() {
    setKeywordInput('');
    updateParams({ q: null, group: null, git: null, tag: null, wt: null, ws: null, forge: null, sort: null });
  }

  function switchMode(next: 'table' | 'tree') {
    updateParams({ view: next === 'tree' ? 'tree' : null });
  }

  function toggleTreeNode(path: string) {
    setTreeExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  }

  function openProject(path: string, opener: string, dir?: string) {
    setOpenError('');
    open.run(opener, path, dir, { onError: (e) => setOpenError(`打开失败：${e.message}`) });
  }

  const error = list.error ? `加载失败：${list.error.message}` : openError || '';
  const openerList = openers.data?.list ?? [];

  // 树模式：从当前筛选结果前端构建（根恒展开，其余按 treeExpanded）
  const treeRoot = mode === 'tree' ? buildProjectTree(sorted) : null;
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
          value={keywordInput}
          onChange={(e) => setKeywordInput(e.target.value)}
          placeholder="搜索项目名 / 路径"
          className="w-64"
        />
        <span className="text-xs text-muted-foreground">
          共 <strong className="text-foreground">{filtered.length}</strong> 个
        </span>
        {/* 排序：列表模式走表头点击（SortableHead），树模式无表头、此处提供下拉 */}
        {mode === 'tree' && (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button variant="ghost" size="sm" className="text-xs text-muted-foreground">
                  <ArrowDownWideNarrow data-icon="inline-start" />
                  排序：
                  {sortMode === 'default'
                    ? '默认'
                    : (sortKeys.find((k) => sortMode === k.value || sortMode === `${k.value}-desc`)?.label ?? '') +
                      (sortMode.endsWith('-desc') ? ' ↓' : '')}
                </Button>
              }
            />
            <DropdownMenuContent align="start">
              {sortKeys.flatMap((k) => [
                <DropdownMenuItem key={k.value} onClick={() => updateParams({ sort: k.value })}>
                  {k.label} ↑
                </DropdownMenuItem>,
                <DropdownMenuItem key={`${k.value}-desc`} onClick={() => updateParams({ sort: `${k.value}-desc` })}>
                  {k.label} ↓
                </DropdownMenuItem>,
              ])}
              {/* sort 缺省即「最近使用 ↓」（默认排序），扫描原序需显式选择 */}
              <DropdownMenuItem onClick={() => updateParams({ sort: 'default' })}>扫描原序</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
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
        <FilterRow label="group" mode="多选">
          <Chip active={groupFilter.length === 0} onClick={() => updateParams({ group: null })}>
            全部
          </Chip>
          {groups.map((g) => (
            <Chip key={g} active={groupFilter.includes(g)} onClick={() => toggleGroup(g)}>
              <span className="flex items-center gap-1">
                {renderIcon(groupIcons.get(g), null)}
                {g}
              </span>
            </Chip>
          ))}
        </FilterRow>
        <FilterRow label="git" mode="单选">
          {gitFilters.map((s) => (
            <Chip key={s.value} active={gitFilter === s.value} onClick={() => setGitFilterValue(s.value)}>
              {s.label}
            </Chip>
          ))}
        </FilterRow>
        {forgeList.length > 0 && (
          <FilterRow label="forge" mode="单选">
            <Chip active={forgeFilter === 'all'} onClick={() => setForgeFilterValue('all')}>
              全部
            </Chip>
            {forgeList.map((f) => (
              <Chip key={f.host} active={forgeFilter === f.host} onClick={() => setForgeFilterValue(f.host)}>
                <span className="flex items-center gap-1">
                  {renderIcon(f.icon ? { type: f.icon.type, value: f.icon.value } : undefined, null)}
                  {f.host}
                </span>
              </Chip>
            ))}
            <Chip active={forgeFilter === 'other'} onClick={() => setForgeFilterValue('other')}>
              其他forge
            </Chip>
            <Chip active={forgeFilter === 'none'} onClick={() => setForgeFilterValue('none')}>
              无forge
            </Chip>
          </FilterRow>
        )}
        <FilterRow label="worktree" mode="单选">
          <Chip active={!wtFilter} onClick={() => setWtFilter(false)}>
            全部
          </Chip>
          <Chip active={wtFilter} onClick={() => setWtFilter(true)}>
            有 <GitBranch className="ml-0.5 inline size-3" />
          </Chip>
        </FilterRow>
        <FilterRow label="workspace" mode="单选">
          <Chip active={!wsFilter} onClick={() => setWsFilter(false)}>
            全部
          </Chip>
          <Chip active={wsFilter} onClick={() => setWsFilter(true)}>
            有 <Layers className="ml-0.5 inline size-3" />
          </Chip>
        </FilterRow>
        {tags.length > 0 && (
          <FilterRow label="tag" mode="单选">
            <Chip active={tagFilter === 'all'} onClick={() => setTagFilterValue('all')}>
              全部
            </Chip>
            {tags.map((t) => (
              <Chip key={t} active={tagFilter === t} onClick={() => setTagFilterValue(t)}>
                {t}
              </Chip>
            ))}
          </FilterRow>
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
                  forgeOf={forgeOf}
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
                  <CycleSortHead
                    label="name"
                    mode={sortMode === 'default' ? null : sortMode}
                    asc="name"
                    desc="name-desc"
                    onSet={(m) => updateParams({ sort: m ?? 'default' })}
                  />
                  <CycleSortHead
                    label="group"
                    mode={sortMode === 'default' ? null : sortMode}
                    asc="group"
                    desc="group-desc"
                    onSet={(m) => updateParams({ sort: m ?? 'default' })}
                    className="w-24"
                  />
                  <TableHead className="w-64">git</TableHead>
                  <CycleSortHead
                    label="最近使用"
                    mode={sortMode === 'default' ? null : sortMode}
                    asc="recent"
                    desc="recent-desc"
                    onSet={(m) => updateParams({ sort: m ?? 'default' })}
                    className="w-32"
                  />
                  <TableHead className="w-32" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {sorted.flatMap((p) => {
                  const targets = projectTargets(p);
                  const multi = targets.length > 1;
                  const expanded = targetExpanded.has(p.path);
                  return [
                    <TableRow key={p.path} className="cursor-pointer" onClick={() => setDrawer(p)}>
                      <TableCell>
                        <div className="flex items-center gap-1.5">
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-1.5">
                              <button
                                type="button"
                                className="text-left text-xs font-medium hover:underline"
                                onClick={() => setDrawer(p)}
                              >
                                {p.name}
                              </button>
                              <ForgeIcon matches={forgeOf(p)} />
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
                              <WorktreeCountBadge p={p} />
                            </div>
                            <div className="mt-0.5 font-mono text-xs text-muted-foreground" title={p.path}>
                              {prettyPath(p.path, home)}
                            </div>
                          </div>
                          {multi && (
                            <button
                              type="button"
                              className="ml-1 shrink-0 text-muted-foreground hover:text-foreground"
                              title={expanded ? '收起目标' : `展开 ${targets.length} 个目标`}
                              aria-label={expanded ? `收起 ${p.name} 目标` : `展开 ${p.name} 目标`}
                              aria-expanded={expanded}
                              onClick={(e) => {
                                e.stopPropagation();
                                toggleTargetExpanded(p.path);
                              }}
                            >
                              <ChevronRight className={cn('size-4 transition-transform', expanded && 'rotate-90')} />
                            </button>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <ClickBadge
                          variant="secondary"
                          title={`筛选 group：${p.group}`}
                          onClick={() => toggleGroupSolo(p.group)}
                        >
                          <span className="flex items-center gap-1">
                            {renderIcon(groupIcons.get(p.group), null)}
                            {p.group}
                          </span>
                        </ClickBadge>
                      </TableCell>
                      <TableCell>
                        <GitCell p={p} onFilter={toggleGitSolo} />
                      </TableCell>
                      <TableCell>{p.lastUsedAt && <LastUsedTime iso={p.lastUsedAt} />}</TableCell>
                      <TableCell onClick={(e) => e.stopPropagation()}>
                        <div className="flex justify-end">
                          <ProjectActions p={p} openerList={openerList} open={open} onOpen={openProject} />
                        </div>
                      </TableCell>
                    </TableRow>,
                    ...(expanded
                      ? targets.map((t) => (
                          <TableRow key={p.path + '\x00' + (t.dir || 'root')} className="text-muted-foreground">
                            <TableCell />
                            <TableCell>
                              <div className="flex items-center gap-1.5 pl-6 text-xs">
                                <TargetKindIcon kind={t.kind} />
                                <span className="shrink-0" title={t.dir || p.path}>
                                  {t.label}
                                </span>
                                <span className="truncate font-mono text-[10px] opacity-70" title={t.dir || p.path}>
                                  {prettyPath(t.dir || p.path, home)}
                                </span>
                              </div>
                            </TableCell>
                            <TableCell />
                            <TableCell />
                            <TableCell />
                            <TableCell onClick={(e) => e.stopPropagation()}>
                              <div className="flex justify-end">
                                <TargetRowActions
                                  p={p}
                                  target={t}
                                  openerList={openerList}
                                  open={open}
                                  onOpen={openProject}
                                />
                              </div>
                            </TableCell>
                          </TableRow>
                        ))
                      : []),
                  ];
                })}
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