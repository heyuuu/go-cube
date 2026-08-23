import { ChevronDown, Copy, Eye, EyeOff, GitBranch, Monitor } from 'lucide-react';
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
import { cn } from '@/lib/utils';
import { quickOpens } from '@/pages/projects/shared';
import { useOpenerList, useOpenerOpen } from '@/queries/project';
import {
  useWorkbenchCommits,
  useWorkbenchInfo,
  useWorkbenchRefs,
  useWorkbenchWorktrees,
  type CommitEntry,
  type WorktreeStatus,
} from '@/queries/workbench';

import { computeGraph, type GraphWire, type LaneInfo } from '../graph-layout';
import {
  refShortName,
  sameSource,
  selectDiffSide,
  selectSource,
  type TreeSource,
  type WorkbenchParams,
} from '../params';
import { useWorktreeVisibility } from '../worktree-visibility';

// git 树面板（提案 1011）：工作台默认入口，取代 SourceTree 的核心视图。
// 上段 = 工作副本状态区（worktree 分组，各自分支/ahead-behind/脏状态）；
// 下段 = commit 图（前端本地 active-lanes 布局 + SVG 拓扑 + 无限滚动；
// dirty 工作副本以虚拟节点挂在各自 HEAD 上方，clean 的以徽标装饰 HEAD 行）。
// 核心交互「选择」：单击 = 单选（source）；cmd/ctrl 单击 = 追加双选（left/right）。
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
        />
      </div>
      <CommitGraphSection path={path} params={params} focusTick={focusTick} hiddenWorktrees={hiddenWorktrees} />
    </div>
  );
}

// --- 工作副本状态区 ---

function WorktreeSection({
  path,
  params,
  onBranchPicked,
  hiddenWorktrees,
  onToggleWorktree,
}: {
  path: string;
  params: WorkbenchParams;
  onBranchPicked: () => void;
  hiddenWorktrees: Set<string>;
  onToggleWorktree: (wtPath: string) => void;
}) {
  const refs = useWorkbenchRefs(path);
  const worktrees = useWorkbenchWorktrees(path);

  return (
    <>
      <Section title="工作副本" icon={<Monitor className="size-3.5" />}>
        {(worktrees.data ?? []).map((wt) => (
          <WorktreeRow
            key={wt.path}
            wt={wt}
            params={params}
            afterSelect={onBranchPicked}
            hidden={hiddenWorktrees.has(wt.path)}
            onToggle={() => onToggleWorktree(wt.path)}
          />
        ))}
      </Section>
      <Section title="分支" icon={<GitBranch className="size-3.5" />}>
        {(refs.data?.locals ?? []).map((b) => (
          <SelectableRow
            key={b}
            label={refShortName(b)}
            source={{ type: 'ref', id: b }}
            params={params}
            badge={b === refs.data?.head ? '当前' : undefined}
            afterSelect={onBranchPicked}
          />
        ))}
      </Section>
    </>
  );
}

function WorktreeRow({
  wt,
  params,
  afterSelect,
  hidden,
  onToggle,
}: {
  wt: WorktreeStatus;
  params: WorkbenchParams;
  afterSelect?: () => void;
  hidden: boolean;
  onToggle: () => void;
}) {
  const src: TreeSource = { type: 'worktree', id: wt.path };
  const name = wt.path.split('/').pop() || wt.path;

  return (
    <div className={cn('flex items-center', hidden && 'opacity-50')}>
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
      <WorktreeOpenActions path={wt.path} name={name} />
    </div>
  );
}

// 副本行尾的 opener 动作（与 projects 页行内动作同构）：已配置的快捷图标 + 全量下拉。
// 须与 SelectableRow（button）并列——HTML 不允许 button 嵌套 button
function WorktreeOpenActions({ path, name }: { path: string; name: string }) {
  const openers = useOpenerList();
  const open = useOpenerOpen();
  const openerList = openers.data?.list ?? [];
  const openerNames = new Set(openerList.map((op) => op.name));
  const onOpen = (app: string) => open.mutate({ path, app });

  return (
    <div className="flex shrink-0 items-center gap-0.5">
      {quickOpens
        .filter((q) => openerNames.has(q.opener))
        .map((q) => (
          <Button
            key={q.opener}
            variant="ghost"
            size="icon-sm"
            title={q.title}
            aria-label={`${q.title}（${name}）`}
            disabled={open.isPending && open.variables?.app === q.opener}
            onClick={() => onOpen(q.opener)}
          >
            {q.icon}
          </Button>
        ))}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label={`打开 ${name}`} />}>
          <ChevronDown className="size-3.5" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-32">
          <DropdownMenuItem onClick={() => void navigator.clipboard.writeText(path)}>
            <Copy className="mr-1 size-3" />
            复制绝对路径
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          {/* Base UI 的 GroupLabel 必须包在 Group 内，否则运行时抛 MenuGroupContext missing */}
          <DropdownMenuGroup>
            <DropdownMenuLabel>打开方式</DropdownMenuLabel>
            {openerList.map((op) => (
              <DropdownMenuItem key={op.name} onClick={() => onOpen(op.name)}>
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
}: {
  path: string;
  params: WorkbenchParams;
  focusTick: number;
  hiddenWorktrees: Set<string>;
}) {
  const commits = useWorkbenchCommits(path);
  const sentinelRef = useRef<HTMLDivElement>(null);

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

    const { nodes, wires } = computeGraph([...virtual, ...decorated]);
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
  }, [commits.data, worktrees.data, hiddenWorktrees]);

  // 定位/高亮的统一目标行：ref → refs 装饰（短名）所在行；worktree → dirty 的虚拟节点行
  // / clean 的 HEAD 行；commit → 自身。选中态高亮不能只比对 source（ref/worktree 与
  // 行上的 commit source 永不相等），定位与高亮共用 focusSha 才能对准同一行。
  const focusSha = useMemo(() => {
    const src = params.source;
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
  }, [params.source, rows, worktrees.data]);

  // 点击定位：滚到 focusSha 行；目标行未加载时自动翻页寻找
  // （无限滚动覆盖不到「未滚动就选中」的场景），无更多页则放弃。
  // focusSha 为 null 有两种含义：无选中（source 也空，直接返回）；
  // 或 ref 的 tip 行尚未加载（memo 在 rows 里找不到）——后者要继续翻页。
  const scrollRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!focusSha && !params.source) return;
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
  }, [params.source, focusSha, focusTick, rows, commits]);

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
        />
      ))}
      <div ref={sentinelRef} className="h-8" />
      {commits.isFetchingNextPage ? (
        <div className="pb-2 text-center text-xs text-muted-foreground">加载中…</div>
      ) : !commits.hasNextPage && rows.length > 0 ? (
        <div className="pb-2 text-center text-xs text-muted-foreground">— 没有更多了 —</div>
      ) : null}
    </div>
  );
}

function CommitRow({
  c,
  laneWidth,
  wires,
  virtualLaneEnd,
  params,
  active,
}: {
  c: RowCommit;
  laneWidth: number;
  wires: GraphWire[];
  virtualLaneEnd: Map<number, number>;
  params: WorkbenchParams;
  active?: boolean; // 选中的是 ref/worktree 时，其 tip/HEAD 所在行
}) {
  const isVirtual = 'worktree' in c && !!c.worktree;
  const nodeColor = isVirtual ? VIRTUAL_COLOR : LANE_PALETTE[(c.color ?? 0) % LANE_PALETTE.length];
  return (
    // 行高用固定 px（与 svg 的 ROW_H 同源）：根字号随视口 clamp 缩放，
    // rem 行高会与大屏下的 svg px 几何错位
    <div data-sha={c.sha} className="flex items-stretch" style={{ height: ROW_H }}>
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
        // 不跟泳道色——泳道色属于「分支线」，标签属于「引用」两个维度
        prefixBadges={
          <>
            {(c.refs ?? []).slice(0, 3).map((r) => (
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
    </div>
  );
}

// ref 徽标按类型分组配色（与泳道色无关）：本地分支蓝、远程灰、tag 琥珀、HEAD 紫
const REF_BADGE_STYLE = {
  local: 'border-blue-500/40 text-blue-600 dark:text-blue-400',
  remote: 'border-muted-foreground/30 text-muted-foreground',
  tag: 'border-amber-500/40 text-amber-600 dark:text-amber-400',
  head: 'border-violet-500/40 text-violet-600 dark:text-violet-400',
  worktree: 'border-cyan-500/40 text-cyan-600 dark:text-cyan-400',
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
            selectSource(next, source);
            afterSelect?.();
          }
          return next;
        },
        { replace: true },
      );
    },
    [source, setSearchParams, afterSelect],
  );

  const isSource = sameSource(params.source, source);
  const isLeft = sameSource(params.left, source);
  const isRight = sameSource(params.right, source);
  const selected = active || isSource || isLeft || isRight;

  return (
    <button
      type="button"
      title={title}
      onClick={handleClick}
      className={cn(
        'min-w-0 flex-1 flex items-center gap-1.5 px-2 text-left text-xs transition-colors hover:bg-accent',
        fixedRow ? 'h-full' : 'leading-7',
        selected && 'bg-primary/15',
      )}
      style={laneColor && selected ? { boxShadow: `inset 2px 0 0 ${laneColor}` } : undefined}
    >
      {mono ? <span className="shrink-0 font-mono text-[10px] text-muted-foreground">{mono}</span> : null}
      {prefixBadges}
      <span className={cn('truncate', selected ? 'font-medium' : undefined)}>{label}</span>
      {badge ? <Badge variant="secondary">{badge}</Badge> : null}
      {badges}
      {isLeft ? <Badge className="ml-auto shrink-0">左</Badge> : null}
      {isRight ? <Badge className="ml-auto shrink-0">右</Badge> : null}
      {time ? (
        <span className="ml-auto shrink-0 text-[10px] text-muted-foreground" title={formatCommitTime(time).full}>
          {formatCommitTime(time).text}
        </span>
      ) : null}
    </button>
  );
}

function Section({ title, icon, children }: { title: string; icon: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="p-2">
      <div className="flex items-center gap-1.5 px-1 pb-1.5 text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
        {icon}
        {title}
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
