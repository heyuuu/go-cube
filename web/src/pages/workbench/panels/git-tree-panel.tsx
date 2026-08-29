import {
  Check,
  ChevronRight,
  Cloud,
  Copy,
  ExternalLink,
  Eye,
  EyeOff,
  GitBranch,
  GitFork,
  Monitor,
  Plus,
  RotateCcw,
  Trash2,
  Ellipsis,
} from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent } from 'react';
import { useSearchParams } from 'react-router';

import { ErrorBanner } from '@/components/error-banner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { renderIcon } from '@/lib/icon';
import { cn } from '@/lib/utils';
import { useOpenerList, useOpenerOpen } from '@/queries/project';
import {
  useWorkbenchCommits,
  useWorkbenchInfo,
  useWorkbenchRefs,
  useWorkbenchRemotes,
  useWorkbenchWorktrees,
  type CommitEntry,
  type RemoteEntry,
  type WorktreeStatus,
} from '@/queries/workbench';

import { computeGraph, simplifyLite, type GraphWire, type LaneInfo } from '../graph-layout';
import {
  refShortName,
  sameSource,
  selectDiffSide,
  selectCurrent,
  type TreeSource,
  type WorkbenchParams,
} from '../params';
import { useWorktreeVisibility } from '../worktree-visibility';

import {
  BranchAddDialog,
  BranchDeleteDialog,
  WorktreeAddDialog,
  WorktreeRemoveDialog,
  WorktreeResetDialog,
} from './worktree-write';

// git 树面板（提案 1011）：工作台默认入口，取代 SourceTree 的核心视图。
// 上段 = 工作副本状态区（worktree 分组，各自分支/ahead-behind/脏状态）；
// 下段 = commit 图（前端本地 active-lanes 布局 + SVG 拓扑 + 无限滚动；
// dirty 工作副本以虚拟节点挂在各自 HEAD 上方，clean 的以徽标装饰 HEAD 行）。
// 核心交互「选择」：单击 = 设 current；cmd/ctrl 单击 = 追加双选（先 base 后 current）。
// 全部选中态写 URL（params 模块统一管理），本面板只是 URL 的渲染者。

export function GitTreePanel({ params }: { params: WorkbenchParams }) {
  const { path } = params;
  const info = useWorkbenchInfo(path);
  // 点击分支的定位信号：即使重复点同一分支（选中值不变）也要重新定位+闪烁
  const [focusTick, setFocusTick] = useState(0);
  // 工作副本显隐开关（默认全展示）：关闭的副本不注入 commit 图——dirty 的虚拟节点、
  // clean 的 HEAD 行装饰都不显示。纯视图过滤，不进 URL、不影响选中态；
  // 持久化按 path 隔离存 localStorage（见 worktree-visibility.ts）
  const { hiddenWorktrees, toggleWorktree } = useWorktreeVisibility(path);

  // 写侧对话框（1031）：面板内多个入口（副本区 + / 分支行 / commit 行）共用
  const [addPrefill, setAddPrefill] = useState<{ branch?: string; commitish?: string } | null>(null);
  const [removeTarget, setRemoveTarget] = useState<WorktreeStatus | null>(null);
  const [deleteBranchName, setDeleteBranchName] = useState<string | null>(null);
  const [branchAddOpen, setBranchAddOpen] = useState(false);
  const worktreesForDialogs = useWorkbenchWorktrees(path);
  const mainPath = worktreesForDialogs.data?.[0]?.path ?? path; // 副本列表主目录在前

  if (info.isPending) {
    return <div className="p-3 text-xs text-muted-foreground">加载中…</div>;
  }
  if (info.isError) {
    return <ErrorBanner message={info.error.message} />;
  }

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <div className="shrink-0 overflow-y-auto border-b border-border">
        <WorktreeSection
          path={path}
          params={params}
          onBranchPicked={() => setFocusTick((n) => n + 1)}
          hiddenWorktrees={hiddenWorktrees}
          onToggleWorktree={toggleWorktree}
          onAddWorktree={setAddPrefill}
          onRemoveWorktree={setRemoveTarget}
          onDeleteBranch={(name) => setDeleteBranchName(name)}
          onAddBranch={() => setBranchAddOpen(true)}
        />
      </div>
      <CommitGraphSection
        path={path}
        params={params}
        focusTick={focusTick}
        hiddenWorktrees={hiddenWorktrees}
        onAddWorktree={setAddPrefill}
      />
      {addPrefill ? <WorktreeAddDialog path={path} prefill={addPrefill} onClose={() => setAddPrefill(null)} /> : null}
      {removeTarget ? (
        <WorktreeRemoveDialog path={path} wt={removeTarget} mainPath={mainPath} onClose={() => setRemoveTarget(null)} />
      ) : null}
      {deleteBranchName ? (
        <BranchDeleteDialog path={path} branch={deleteBranchName} onClose={() => setDeleteBranchName(null)} />
      ) : null}
      {branchAddOpen ? <BranchAddDialog path={path} onClose={() => setBranchAddOpen(false)} /> : null}
    </div>
  );
}

// workspace 子行展开态持久化（与 worktree-visibility 同款模式）
const WS_EXPANDED_KEY = 'cube.workbench.wsExpanded';
function loadWsExpanded(): Set<string> {
  try {
    return new Set<string>(JSON.parse(localStorage.getItem(WS_EXPANDED_KEY) ?? '[]'));
  } catch {
    return new Set<string>();
  }
}

// commit 图展示模式持久化（与 wsExpanded 同款模式）：
// full = 全量提交；lite = 轻量拓扑（只留 ref/merge/分叉点，见 simplifyLite）
const GRAPH_MODE_KEY = 'cube.workbench.gitTree.mode';
function loadGraphMode(): 'full' | 'lite' {
  return localStorage.getItem(GRAPH_MODE_KEY) === 'lite' ? 'lite' : 'full';
}

// --- 工作副本状态区 ---

function WorktreeSection({
  path,
  params,
  onBranchPicked,
  hiddenWorktrees,
  onToggleWorktree,
  onAddWorktree,
  onRemoveWorktree,
  onDeleteBranch,
  onAddBranch,
}: {
  path: string;
  params: WorkbenchParams;
  onBranchPicked: () => void;
  hiddenWorktrees: Set<string>;
  onToggleWorktree: (wtPath: string) => void;
  onAddWorktree: (prefill: { branch?: string; commitish?: string }) => void;
  onRemoveWorktree: (wt: WorktreeStatus) => void;
  onDeleteBranch: (name: string) => void;
  onAddBranch: () => void;
}) {
  const refs = useWorkbenchRefs(path);
  const worktrees = useWorkbenchWorktrees(path);

  return (
    <>
      <Section
        title="工作副本"
        icon={<Monitor className="size-3.5" />}
        action={
          <Button
            variant="ghost"
            size="icon-sm"
            title="新建 worktree"
            aria-label="新建 worktree"
            onClick={() => onAddWorktree({})}
          >
            <Plus className="size-3.5" />
          </Button>
        }
      >
        {(worktrees.data ?? []).map((wt) => (
          <WorktreeRow
            key={wt.path}
            wt={wt}
            params={params}
            afterSelect={onBranchPicked}
            hidden={hiddenWorktrees.has(wt.path)}
            onToggle={() => onToggleWorktree(wt.path)}
            onRemove={() => onRemoveWorktree(wt)}
          />
        ))}
      </Section>
      <RemoteSection path={path} />
      <Section
        title="分支"
        icon={<GitBranch className="size-3.5" />}
        action={
          <Button variant="ghost" size="icon-sm" title="新建分支" aria-label="新建分支" onClick={onAddBranch}>
            <Plus className="size-3.5" />
          </Button>
        }
      >
        {(refs.data?.locals ?? []).map((b) => (
          <BranchRow
            key={b}
            refName={b}
            isHead={b === refs.data?.head}
            params={params}
            afterSelect={onBranchPicked}
            onAddWorktree={() => onAddWorktree({ branch: refShortName(b) })}
            onDeleteBranch={() => onDeleteBranch(refShortName(b))}
          />
        ))}
      </Section>
    </>
  );
}

// --- 远端分组：remote 配置列表（非 ref，不可选中），复制地址 + 跳转托管平台网页 ---

function RemoteSection({ path }: { path: string }) {
  const remotes = useWorkbenchRemotes(path);
  const list = remotes.data ?? [];
  if (list.length === 0) return null;
  return (
    <Section title="远端" icon={<Cloud className="size-3.5" />}>
      {list.map((r) => (
        <RemoteRow key={r.name} remote={r} />
      ))}
    </Section>
  );
}

function RemoteRow({ remote }: { remote: RemoteEntry }) {
  const [copied, setCopied] = useState(false);
  const copy = () => {
    void navigator.clipboard.writeText(remote.url).then(() => {
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    });
  };
  return (
    <div className="flex items-center px-2 leading-7 text-xs">
      <span className="shrink-0 font-medium">{remote.name}</span>
      <span className="ml-2 min-w-0 truncate font-mono text-[10px] text-muted-foreground" title={remote.url}>
        {remote.url}
      </span>
      <Button
        variant="ghost"
        size="icon-sm"
        className="ml-auto shrink-0"
        title="复制 git 地址"
        aria-label={`复制 ${remote.name} 的地址`}
        onClick={copy}
      >
        {copied ? <Check className="size-3.5 text-green-600" /> : <Copy className="size-3.5" />}
      </Button>
      {remote.webUrl ? (
        <Button
          variant="ghost"
          size="icon-sm"
          className="shrink-0"
          title={remote.webUrl}
          aria-label={`打开 ${remote.name} 的网页`}
          onClick={() => window.open(remote.webUrl, '_blank', 'noopener')}
        >
          <ExternalLink className="size-3.5" />
        </Button>
      ) : null}
    </div>
  );
}

// 分支行：可选中主体 + 尾部 hover 动作（在此新建 worktree / 删除分支）。
// 与工作副本行同构：动作按钮与 SelectableRow（button）并列，不能嵌套
function BranchRow({
  refName,
  isHead,
  params,
  afterSelect,
  onAddWorktree,
  onDeleteBranch,
}: {
  refName: string;
  isHead: boolean;
  params: WorkbenchParams;
  afterSelect?: () => void;
  onAddWorktree: () => void;
  onDeleteBranch: () => void;
}) {
  // 选中态放整行容器（与 WorktreeRow 同构）：SelectableRow 传 bare 后自身不上底色
  const src: TreeSource = { type: 'ref', id: refName };
  const selected = sameSource(params.current, src) || sameSource(params.base, src);
  return (
    <div className={cn('group flex items-center hover:bg-accent', selected && 'bg-primary/15')}>
      <SelectableRow
        label={refShortName(refName)}
        source={{ type: 'ref', id: refName }}
        params={params}
        badge={isHead ? '当前' : undefined}
        afterSelect={afterSelect}
        bare
      />
      <div className="flex shrink-0 items-center opacity-0 transition-opacity group-hover:opacity-100">
        <Button
          variant="ghost"
          size="icon-sm"
          title="在此分支新建 worktree"
          aria-label={`在 ${refShortName(refName)} 新建 worktree`}
          onClick={onAddWorktree}
        >
          <Plus className="size-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="icon-sm"
          title="删除分支"
          aria-label={`删除分支 ${refShortName(refName)}`}
          onClick={onDeleteBranch}
        >
          <Trash2 className="size-3.5" />
        </Button>
      </div>
    </div>
  );
}

function WorktreeRow({
  wt,
  params,
  afterSelect,
  hidden,
  onToggle,
  onRemove,
}: {
  wt: WorktreeStatus;
  params: WorkbenchParams;
  afterSelect?: () => void;
  hidden: boolean;
  onToggle: () => void;
  onRemove: () => void;
}) {
  const src: TreeSource = { type: 'worktree', id: wt.path };
  const name = wt.path.split('/').pop() || wt.path;
  // hover/选中态放整行容器（前后的开关/opener 按钮同属一行，只亮中间段会很碎）
  const selected = sameSource(params.current, src) || sameSource(params.base, src);

  // workspace 子行展开态：与副本显隐开关同款 localStorage 持久化
  const wsList = wt.workspaces ?? [];
  const [resetOpen, setResetOpen] = useState(false);
  const [wsExpanded, setWsExpanded] = useState(() => loadWsExpanded().has(wt.path));
  const toggleWs = () => {
    const next = new Set(loadWsExpanded());
    if (next.has(wt.path)) next.delete(wt.path);
    else next.add(wt.path);
    localStorage.setItem(WS_EXPANDED_KEY, JSON.stringify([...next]));
    setWsExpanded(next.has(wt.path));
  };

  return (
    <div className="w-full">
      <div
        className={cn(
          'group flex items-center transition-colors hover:bg-accent',
          selected && 'bg-primary/15',
          hidden && 'opacity-50',
        )}
      >
        {wsList.length > 0 && (
          <Button
            variant="ghost"
            size="icon-sm"
            className="shrink-0"
            title={wsExpanded ? '收起 workspace' : `展开 ${wsList.length} 个 workspace`}
            aria-label={wsExpanded ? `收起 ${name} 的 workspace` : `展开 ${name} 的 workspace`}
            aria-expanded={wsExpanded}
            onClick={toggleWs}
          >
            <ChevronRight className={cn('size-3.5 transition-transform', wsExpanded && 'rotate-90')} />
          </Button>
        )}
        <Button
          variant="ghost"
          size="icon-sm"
          title={hidden ? '在 commit 图中展示该副本' : '在 commit 图中隐藏该副本'}
          aria-label={hidden ? `展示 ${name}` : `隐藏 ${name}`}
          onClick={onToggle}
        >
          {hidden ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
        </Button>
        <SelectableRow
          label={name}
          source={src}
          params={params}
          title={wt.path}
          afterSelect={afterSelect}
          bare
          badges={
            <>
              {wt.branch ? <Badge variant="secondary">{wt.branch}</Badge> : null}
              {wt.bare ? <Badge variant="outline">bare</Badge> : null}
              {wt.ahead > 0 ? <Badge variant="secondary">↑{wt.ahead}</Badge> : null}
              {wt.behind > 0 ? <Badge variant="secondary">↓{wt.behind}</Badge> : null}
              {wt.dirty ? (
                <Badge variant="destructive" className="px-1">
                  脏 {wt.staged + wt.unstaged + wt.untracked}
                </Badge>
              ) : null}
              {wt.detached ? <Badge variant="outline">detached</Badge> : null}
            </>
          }
        />
        <div className="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity hover:opacity-100 group-hover:opacity-100">
          <Button variant="ghost" size="icon-sm" title="删除该 worktree" aria-label={`删除 ${name}`} onClick={onRemove}>
            <Trash2 className="size-3.5" />
          </Button>
        </div>
        <WorktreeOpenActions path={wt.path} name={name} onReset={() => setResetOpen(true)} />
      </div>
      {resetOpen ? (
        <WorktreeResetDialog wtPath={wt.path} branch={wt.branch || undefined} onClose={() => setResetOpen(false)} />
      ) : null}
      {wsExpanded &&
        wsList.map((w) => {
          // 子行不可选中（不是 TreeSource，纯打开入口）：目录名 + 相对路径 + 各自的打开动作
          const wsDir = wt.path.replace(/\/$/, '') + '/' + w.path;
          return (
            <div key={wsDir} className="flex items-center pl-8 text-xs text-muted-foreground hover:bg-accent">
              <span className="shrink-0">{w.name}</span>
              <span className="ml-2 min-w-0 truncate font-mono text-[10px] opacity-70" title={wsDir}>
                {w.path}
              </span>
              <div className="ml-auto flex shrink-0 items-center">
                <WorktreeOpenActions path={wsDir} name={w.name} />
              </div>
            </div>
          );
        })}
    </div>
  );
}

// 副本行尾的 opener 动作（与 projects 页行内动作同构）：已配置的快捷图标 + 全量下拉。
// 快捷清单本面板自维护（不含 cube-workbench——已在工作台内，没必要再跳工作台）。
// 须与 SelectableRow（button）并列——HTML 不允许 button 嵌套 button
const QUICK_OPENS = ['finder', 'stree'];

// onReset 仅 worktree 行传入（reset 作用于整个副本，workspace 子目录不适用）
function WorktreeOpenActions({ path, name, onReset }: { path: string; name: string; onReset?: () => void }) {
  const openers = useOpenerList();
  const open = useOpenerOpen();
  const openerList = openers.data?.list ?? [];
  const openerNames = new Set(openerList.map((op) => op.name));
  const openerByName = new Map(openerList.map((op) => [op.name, op]));
  const onOpen = (opener: string) => open.mutate({ path, opener });

  return (
    <div className="flex shrink-0 items-center gap-0.5">
      {QUICK_OPENS.filter((openerName) => openerNames.has(openerName)).map((openerName) => {
        const op = openerByName.get(openerName);
        if (!op) return null;
        return (
          <Button
            key={openerName}
            variant="ghost"
            size="icon-sm"
            title={op.title}
            aria-label={`${op.title}（${name}）`}
            disabled={open.isPending && open.variables?.opener === openerName}
            onClick={() => onOpen(openerName)}
          >
            {renderIcon(op?.icon, null)}
          </Button>
        );
      })}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label={`打开 ${name}`} />}>
          <Ellipsis className="size-3.5" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-auto min-w-56">
          <DropdownMenuItem onClick={() => void navigator.clipboard.writeText(path)}>
            <Copy className="mr-1 size-3" />
            复制绝对路径
          </DropdownMenuItem>
          {onReset ? (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={onReset}>
                <RotateCcw className="mr-1 size-3" />
                重置到指定位置
              </DropdownMenuItem>
            </>
          ) : null}
          <DropdownMenuSeparator />
          {/* Base UI 的 GroupLabel 必须包在 Group 内，否则运行时抛 MenuGroupContext missing */}
          <DropdownMenuGroup>
            <DropdownMenuLabel>打开方式</DropdownMenuLabel>
            {openerList.map((op) => (
              <DropdownMenuItem key={op.name} onClick={() => onOpen(op.name)}>
                {renderIcon(op?.icon, null)}
                {op.title}
              </DropdownMenuItem>
            ))}
            {openerList.length === 0 && <div className="px-2 py-1.5 text-xs text-muted-foreground">未配置 opener</div>}
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

// 虚拟节点：dirty worktree 的「未提交状态」合成提交（父 = 该副本 HEAD，无 msg，时间 = 当前）；
// 选中即浏览该工作副本现场（worktree 源）
type WorktreeNode = CommitEntry & { worktree: WorktreeStatus };
type RowCommit = (CommitEntry | WorktreeNode) & LaneInfo;

// --- commit 图 ---

function CommitGraphSection({
  path,
  params,
  focusTick,
  hiddenWorktrees,
  onAddWorktree,
}: {
  path: string;
  params: WorkbenchParams;
  focusTick: number;
  hiddenWorktrees: Set<string>;
  onAddWorktree: (prefill: { branch?: string; commitish?: string }) => void;
}) {
  const commits = useWorkbenchCommits(path);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const [mode, setMode] = useState<'full' | 'lite'>(loadGraphMode);
  const toggleMode = () => {
    const next = mode === 'full' ? 'lite' : 'full';
    setMode(next);
    localStorage.setItem(GRAPH_MODE_KEY, next);
  };

  // 无限滚动：哨兵进入视口即拉下一页
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;
    const observer = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting && commits.hasNextPage && !commits.isFetchingNextPage) {
        commits.fetchNextPage();
      }
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, [commits]);

  const worktrees = useWorkbenchWorktrees(path);
  // 页拼接去重（skip 分页在仓库有新提交时可能边界重复）→ 注入 worktree 虚拟节点/装饰
  // → 本地算泳道布局。布局永远从「当前持有数据」推导，不存在跨快照拼接错位。
  const { rows, wireMap, virtualLaneEnd } = useMemo(() => {
    const seen = new Set<string>();
    const list: CommitEntry[] = [];
    for (const page of commits.data?.pages ?? []) {
      for (const c of page.list ?? []) {
        if (seen.has(c.sha)) continue;
        seen.add(c.sha);
        list.push({ ...c, parents: c.parents ?? [], refs: c.refs ?? [] });
      }
    }

    // worktree 视作一个 ref：dirty → 指向虚拟节点（内容 = 未提交状态，父 = HEAD）；
    // clean → 直接装饰在 HEAD 提交行上。虚拟节点按 worktree 顺序排在最前（时间 = 当前）
    const decorated = list.map((c) => ({ ...c }));
    const now = Math.floor(Date.now() / 1000);
    const virtual: WorktreeNode[] = [];
    for (const wt of worktrees.data ?? []) {
      if (hiddenWorktrees.has(wt.path)) continue; // 关闭开关的副本不进图（虚拟节点与装饰都不注入）
      if (wt.bare) continue; // bare 无工作区，无未提交概念
      const label = wt.path.split('/').pop() || wt.path;
      if (wt.dirty) {
        virtual.push({
          sha: `worktree:${wt.path}`,
          shortSha: '-------', // 占位对齐：与真实 commit 的短 sha 同列，列表更整齐
          parents: [wt.head],
          author: '',
          timestamp: now,
          refs: [{ name: label, kind: 'worktree' }],
          subject: '',
          worktree: wt,
        });
      } else {
        const hit = decorated.find((c) => c.sha === wt.head);
        if (hit) hit.refs = [...(hit.refs ?? []), { name: label, kind: 'worktree' }];
      }
    }

    const { nodes, wires } = computeGraph([...virtual, ...(mode === 'lite' ? simplifyLite(decorated) : decorated)]);
    const map = new Map<number, GraphWire[]>();
    for (const w of wires) map.set(w.row, [...(map.get(w.row) ?? []), w]);
    // 虚拟节点（未提交改动）独用灰色：与已提交节点一眼区分。
    // 置灰范围只到「虚拟节点 → 其 HEAD」为止：HEAD 之下同泳道的线属于真实历史，
    // 记 lane → HEAD 行号，灰线段判 row < headRow（HEAD 未加载则整段可见线全灰）
    const nodesOf = new Map(nodes.map((n, i) => [n.sha, i]));
    const virtualLaneEnd = new Map<number, number>();
    for (const n of nodes) {
      if (!('worktree' in n && n.worktree)) continue;
      const head = n.parents?.[0];
      const headRow = head != null ? (nodesOf.get(head) ?? nodes.length) : 0;
      virtualLaneEnd.set(n.lane, headRow);
    }
    return { rows: nodes, wireMap: map, virtualLaneEnd };
  }, [commits.data, worktrees.data, hiddenWorktrees, mode]);

  // 定位/高亮的统一目标行：ref → refs 装饰（短名）所在行；worktree → dirty 的虚拟节点行
  // / clean 的 HEAD 行；commit → 自身。选中态高亮不能只比对 source（ref/worktree 与
  // 行上的 commit source 永不相等），定位与高亮共用 focusSha 才能对准同一行。
  const focusSha = useMemo(() => {
    const src = params.current;
    if (!src) return null;
    if (src.type === 'ref') {
      const short = refShortName(src.id);
      return rows.find((r) => (r.refs ?? []).some((x) => x.name === short))?.sha ?? null;
    }
    if (src.type === 'worktree') {
      const wt = (worktrees.data ?? []).find((w) => w.path === src.id);
      if (!wt) return null;
      return wt.dirty ? `worktree:${wt.path}` : wt.head;
    }
    return src.id;
  }, [params.current, rows, worktrees.data]);

  // 点击定位：滚到 focusSha 行；目标行未加载时自动翻页寻找
  // （无限滚动覆盖不到「未滚动就选中」的场景），无更多页则放弃。
  // focusSha 为 null 有两种含义：无选中（source 也空，直接返回）；
  // 或 ref 的 tip 行尚未加载（memo 在 rows 里找不到）——后者要继续翻页。
  const scrollRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!focusSha && !params.current) return;
    const el = scrollRef.current?.querySelector(`[data-sha="${focusSha}"]`);
    if (el) {
      el.scrollIntoView({ block: 'center' });
      // 定位闪烁：短暂高亮目标行（class 由命令式添加，React 渲染不冲突，超时移除）
      el.classList.remove('row-flash');
      void (el as HTMLElement).offsetWidth; // 重启动画
      el.classList.add('row-flash');
      window.setTimeout(() => el.classList.remove('row-flash'), 2500);
      return;
    }
    if (commits.hasNextPage && !commits.isFetchingNextPage) {
      void commits.fetchNextPage();
    }
  }, [params.current, focusSha, focusTick, rows, commits]);

  const maxLane = useMemo(() => {
    let m = 0;
    for (const r of rows) m = Math.max(m, r.lane ?? 0);
    for (const ws of wireMap.values()) for (const w of ws) m = Math.max(m, w.from, w.to);
    return m;
  }, [rows, wireMap]);

  if (commits.isError) {
    return (
      <div className="min-h-0 flex-1 overflow-hidden">
        <ErrorBanner message={commits.error.message} />
      </div>
    );
  }

  const laneWidth = LANE_X0 * 2 + (maxLane + 1) * LANE_W;

  return (
    <>
    <div className="flex shrink-0 items-center justify-end border-b border-border px-2 py-1">
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={toggleMode}
        title={mode === 'full' ? '切换为轻量拓扑（只留分支/tag/merge/分叉点）' : '切换为完整提交列表'}
        className={cn('text-muted-foreground', mode === 'lite' && 'text-foreground')}
      >
        <GitFork className="size-3" />
      </Button>
    </div>
    <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto">
      {rows.map((c, i) => (
        <CommitRow
          key={c.sha}
          c={c}
          laneWidth={laneWidth}
          wires={wireMap.get(i - 1) ?? []}
          virtualLaneEnd={virtualLaneEnd}
          params={params}
          active={c.sha === focusSha}
          onAddWorktree={() => onAddWorktree({ commitish: c.sha })}
        />
      ))}
      <div ref={sentinelRef} className="h-8" />
      {commits.isFetchingNextPage ? (
        <div className="pb-2 text-center text-xs text-muted-foreground">加载中…</div>
      ) : !commits.hasNextPage && rows.length > 0 ? (
        <div className="pb-2 text-center text-xs text-muted-foreground">— 没有更多了 —</div>
      ) : null}
    </div>
    </>
  );
}

function CommitRow({
  c,
  laneWidth,
  wires,
  virtualLaneEnd,
  params,
  active,
  onAddWorktree,
}: {
  c: RowCommit;
  laneWidth: number;
  wires: GraphWire[];
  virtualLaneEnd: Map<number, number>;
  params: WorkbenchParams;
  active?: boolean; // 选中的是 ref/worktree 时，其 tip/HEAD 所在行
  onAddWorktree: () => void;
}) {
  const isVirtual = 'worktree' in c && !!c.worktree;
  const nodeColor = isVirtual ? VIRTUAL_COLOR : LANE_PALETTE[(c.color ?? 0) % LANE_PALETTE.length];
  return (
    // 行高用固定 px（与 svg 的 ROW_H 同源）：根字号随视口 clamp 缩放，
    // rem 行高会与大屏下的 svg px 几何错位
    <div data-sha={c.sha} className="group/commit flex items-stretch hover:bg-accent" style={{ height: ROW_H }}>
      {/* 泳道列：本行 svg 画「上一节点中心 → 本节点中心」的连线段（Row = rowIndex-1）+ 本行节点。
          节点圆心在行高中点，线段纵向须跨 -ROW_H/2 ~ +ROW_H/2（上一行圆心到本行圆心），
          svg 设 overflow visible 允许向上越界绘制 */}
      <svg
        width={laneWidth}
        height={ROW_H}
        className="shrink-0 self-center"
        style={{ overflow: 'visible' }}
        shapeRendering="geometricPrecision"
      >
        {wires.map((w, wi) => {
          const x1 = LANE_X0 + w.from * LANE_W;
          const x2 = LANE_X0 + w.to * LANE_W;
          // 虚拟节点的连线段（起点在其泳道、且尚未到其 HEAD 行）置灰
          const end = virtualLaneEnd.get(w.from);
          const color = end !== undefined && w.row < end ? VIRTUAL_COLOR : LANE_PALETTE[w.color % LANE_PALETTE.length];
          // 同泳道 = 竖线；切入/切出行 = 单条斜线（无曲线、无折线），其余位置恒竖线
          return <line key={wi} x1={x1} y1={-ROW_H / 2} x2={x2} y2={ROW_H / 2} stroke={color} strokeWidth={1.5} />;
        })}
        <circle
          cx={LANE_X0 + (c.lane ?? 0) * LANE_W}
          cy={ROW_H / 2}
          r={3.5}
          fill={nodeColor}
          stroke="hsl(var(--background))"
          strokeWidth={1.5}
        />
      </svg>
      <SelectableRow
        label={'worktree' in c && c.worktree ? '未提交改动' : c.subject}
        source={
          'worktree' in c && c.worktree ? { type: 'worktree', id: c.worktree.path } : { type: 'commit', id: c.sha }
        }
        params={params}
        title={
          'worktree' in c && c.worktree
            ? `${c.worktree.path}\n未提交：暂存 ${c.worktree.staged} · 修改 ${c.worktree.unstaged} · 未跟踪 ${c.worktree.untracked}`
            : `${c.sha}\n${c.author}\n${formatCommitTime(c.timestamp).full}`
        }
        mono={c.shortSha || undefined}
        time={c.timestamp}
        laneColor={nodeColor}
        fixedRow
        active={active}
        badges={
          'worktree' in c && c.worktree ? (
            <Badge variant="destructive" className="px-1">
              {c.worktree.staged + c.worktree.unstaged + c.worktree.untracked} 文件
            </Badge>
          ) : undefined
        }
        // 标签挪到 message 前；样式按 ref 类型分组（本地/远程/tag/head），
        // 不跟泳道色——泳道色属于「分支线」，标签属于「引用」两个维度。
        // worktree 徽标排最前（副本是「位置」语义，优先于分支引用）
        prefixBadges={
          <>
            {[...(c.refs ?? [])]
              .sort((a, b) => (a.kind === 'worktree' ? -1 : b.kind === 'worktree' ? 1 : 0))
              .slice(0, 3)
              .map((r) => (
              <Badge
                key={r.kind + r.name}
                variant="outline"
                className={cn(
                  'max-w-24 shrink-0 truncate border px-1 py-0 text-[10px]',
                  REF_BADGE_STYLE[r.kind as keyof typeof REF_BADGE_STYLE] ?? REF_BADGE_STYLE.local,
                )}
              >
                {r.name}
              </Badge>
            ))}
          </>
        }
      />
      {!('worktree' in c && c.worktree) ? (
        <Button
          variant="ghost"
          size="icon-sm"
          className="mr-1 shrink-0 opacity-0 transition-opacity group-hover/commit:opacity-100"
          title="在此 commit 新建 worktree"
          aria-label="在此 commit 新建 worktree"
          onClick={onAddWorktree}
        >
          <Plus className="size-3.5" />
        </Button>
      ) : null}
    </div>
  );
}

// ref 徽标按类型分组配色（与泳道色无关）：本地分支蓝、远程灰、tag 琥珀、HEAD 紫；
// worktree 实心填充（其余仅描边）——副本徽标与引用徽标视觉分层
const REF_BADGE_STYLE = {
  local: 'border-blue-500/40 text-blue-600 dark:text-blue-400',
  remote: 'border-muted-foreground/30 text-muted-foreground',
  tag: 'border-amber-500/40 text-amber-600 dark:text-amber-400',
  head: 'border-violet-500/40 text-violet-600 dark:text-violet-400',
  worktree: 'border-cyan-500/50 bg-cyan-500/15 text-cyan-700 dark:text-cyan-300',
} as const;

// 泳道几何：列宽/行高/左边距；调色板与后端 color 索引对应（循环取色）
const LANE_W = 12;
const ROW_H = 28;
const LANE_X0 = 8;
const LANE_PALETTE = ['#3b82f6', '#f97316', '#10b981', '#ec4899', '#8b5cf6', '#eab308', '#14b8a6', '#ef4444'];
// 未提交虚拟节点与其连线：灰色，与已提交节点区分。
// 须用具体色值——SVG 的 fill/stroke 属性不解析 CSS 变量（hsl(var(--...)) 会失效，
// fill 落回黑、stroke 落回 none），选明暗主题下都可辨的中灰
const VIRTUAL_COLOR = '#9ca3af';

// --- 通用可选中行 ---

type SelectableRowProps = {
  label: string;
  source: TreeSource;
  params: WorkbenchParams;
  title?: string;
  mono?: string;
  badge?: string;
  badges?: React.ReactNode;
  prefixBadges?: React.ReactNode; // 显示在 label 前（commit 行的 ref 标签）
  time?: number;
  laneColor?: string; // 泳道色：选中行以分支色描边
  fixedRow?: boolean; // commit 图行：固定 px 高度（外层行 div 已定高），不用 rem 行高
  active?: boolean; // 选中态外部判定（选中的是 ref/worktree 时，其 tip/HEAD 所在行）
  afterSelect?: () => void; // 单击选中后回调（分支行用于触发重新定位）
  bare?: boolean; // 只承担点击/内容，hover/选中态由外层行容器接管（工作副本行整行高亮）
};

function SelectableRow({
  label,
  source,
  params,
  title,
  mono,
  badge,
  badges,
  prefixBadges,
  time,
  laneColor,
  fixedRow,
  active,
  afterSelect,
  bare,
}: SelectableRowProps) {
  const [, setSearchParams] = useSearchParams();
  const handleClick = useCallback(
    (e: MouseEvent) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          if (e.metaKey || e.ctrlKey) {
            selectDiffSide(next, source);
          } else {
            selectCurrent(next, source);
            afterSelect?.();
          }
          return next;
        },
        { replace: true },
      );
    },
    [source, setSearchParams, afterSelect],
  );

  const isCurrent = sameSource(params.current, source);
  const isBase = sameSource(params.base, source);
  const selected = active || isCurrent || isBase;

  return (
    <button
      type="button"
      title={title}
      onClick={handleClick}
      className={cn(
        'min-w-0 flex-1 flex items-center gap-1.5 px-2 text-left text-xs transition-colors',
        !bare && 'hover:bg-accent',
        fixedRow ? 'h-full' : 'leading-7',
        selected && !bare && 'bg-primary/15',
      )}
      style={laneColor && selected ? { boxShadow: `inset 2px 0 0 ${laneColor}` } : undefined}
    >
      {mono ? <span className="shrink-0 font-mono text-[10px] text-muted-foreground">{mono}</span> : null}
      {prefixBadges}
      <span className={cn('truncate', selected ? 'font-medium' : undefined)}>{label}</span>
      {badge ? <Badge variant="secondary">{badge}</Badge> : null}
      {badges}
      {isBase ? <Badge className="ml-auto shrink-0">基准</Badge> : null}
      {isCurrent ? <Badge className="ml-auto shrink-0">当前</Badge> : null}
      {time ? (
        <span className="ml-auto shrink-0 text-[10px] text-muted-foreground" title={formatCommitTime(time).full}>
          {formatCommitTime(time).text}
        </span>
      ) : null}
    </button>
  );
}

function Section({
  title,
  icon,
  action,
  children,
}: {
  title: string;
  icon: React.ReactNode;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section className="p-2">
      <div className="flex items-center gap-1.5 px-1 pb-1.5 text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
        {icon}
        {title}
        {action ? <div className="ml-auto">{action}</div> : null}
      </div>
      <div className="flex flex-col gap-0.5">{children}</div>
    </section>
  );
}

// 提交时间显示：今天 → 纯时间（HH:mm）；今天以前 → 纯日期（当年 MM-DD，跨年 YYYY-MM-DD）；
// 悬停 → 完整「日期 时间」
function formatCommitTime(ts: number): { text: string; full: string } {
  const d = new Date(ts * 1000);
  const now = new Date();
  const sameDay =
    d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth() && d.getDate() === now.getDate();
  const hm = `${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
  const ymd = sameYear(d, now)
    ? `${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
    : `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
  const full = `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} ${hm}`;
  return { text: sameDay ? hm : ymd, full };
}

function sameYear(a: Date, b: Date): boolean {
  return a.getFullYear() === b.getFullYear();
}

function pad2(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}
