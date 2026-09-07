// git 树面板·commit 图（提案 1011）：前端本地 active-lanes 布局 + SVG 拓扑 + 无限滚动；
// dirty 工作副本以虚拟节点挂在各自 HEAD 上方，clean 的以徽标装饰 HEAD 行。
import { Plus } from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';

import { ErrorBanner } from '@/components/error-banner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { useLocalPref } from '@/hooks/use-local-pref';
import { cn } from '@/lib/utils';
import { useWorkbenchCommits, useWorkbenchWorktrees, type CommitEntry, type WorktreeStatus } from '@/queries/workbench';

import { computeGraph, simplifyLite, type GraphWire, type LaneInfo } from '../graph-layout';

// 虚拟节点：dirty worktree 的「未提交状态」合成提交（父 = 该副本 HEAD，无 msg，时间 = 当前）；
// 选中即浏览该工作副本现场（worktree 源）
type WorktreeNode = CommitEntry & { worktree: WorktreeStatus };
type RowCommit = (CommitEntry | WorktreeNode) & LaneInfo;
import { refShortName, type WorkbenchParams } from '../params';

import { formatCommitTime, REF_BADGE_STYLE } from './commit-bits';
import { SelectableRow } from './git-tree-bits';

// commit 图展示模式持久化（与 wsExpanded 同款模式）：
// full = 全量提交；lite = 轻量拓扑（只留 ref/merge/分叉点，见 simplifyLite）
// LITE_KEEP_RECENT：轻量模式下顶部无条件保留的最近提交数（N 边界画虚线分割；
// 调试期常量，方便改小改大看效果）
const LITE_KEEP_RECENT = 5;
const GRAPH_MODE_KEY = 'cube.workbench.gitTree.mode';
// --- commit 图 ---

export function CommitGraphSection({
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
  const [mode, toggleModeRaw] = useLocalPref<'full' | 'lite'>(GRAPH_MODE_KEY, 'full', (raw) =>
    raw === 'lite' ? 'lite' : 'full',
  );
  const toggleMode = () => toggleModeRaw(mode === 'full' ? 'lite' : 'full');

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
  // 虚拟节点（未提交改动）的展示时间 = 本组件挂载时的「当前」，一次取值不随渲染抖动
  const [nowTs] = useState(() => Math.floor(Date.now() / 1000));
  // 页拼接去重（skip 分页在仓库有新提交时可能边界重复）→ 注入 worktree 虚拟节点/装饰
  // → 本地算泳道布局。布局永远从「当前持有数据」推导，不存在跨快照拼接错位。
  const { rows, wireMap, virtualOriginEnd, dividerRow } = useMemo(() => {
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
          timestamp: nowTs,
          refs: [{ name: label, kind: 'worktree' }],
          subject: '',
          worktree: wt,
        });
      } else {
        const hit = decorated.find((c) => c.sha === wt.head);
        if (hit) hit.refs = [...(hit.refs ?? []), { name: label, kind: 'worktree' }];
      }
    }

    const lite = mode === 'lite' ? simplifyLite(decorated, LITE_KEEP_RECENT) : null;
    const { nodes, wires } = computeGraph([...virtual, ...(lite ?? decorated)]);
    const map = new Map<number, GraphWire[]>();
    for (const w of wires) map.set(w.row, [...(map.get(w.row) ?? []), w]);
    // 虚拟节点（未提交改动）独用灰色：与已提交节点一眼区分。
    // 置灰范围只到「虚拟节点 → 其 HEAD」为止：HEAD 之下的线属于真实历史。
    // 灰线段的键是线段的 origin（线的起始行，见 GraphWire）而非泳道号——
    // 泳道号会被复用（让位留洞、newLane 回填），按泳道号判会把恰好复用到
    // 虚拟泳道号的真实支线（如另一 worktree 的 tip 出线）整段误置灰。
    // 记 origin 行号 → HEAD 行号，灰线段判 row < headRow（HEAD 未加载则整段可见线全灰）
    const nodesOf = new Map(nodes.map((n, i) => [n.sha, i]));
    // 轻量模式「最近 N 个」区域的下边界行（第 N 个提交所在行；列表比 N 长才画）
    const dividerRow =
      lite && decorated.length > LITE_KEEP_RECENT ? (nodesOf.get(decorated[LITE_KEEP_RECENT - 1].sha) ?? -1) : -1;
    const virtualOriginEnd = new Map<number, number>();
    nodes.forEach((n, i) => {
      if (!('worktree' in n && n.worktree)) return;
      const head = n.parents?.[0];
      const headRow = head != null ? (nodesOf.get(head) ?? nodes.length) : 0;
      virtualOriginEnd.set(i, headRow);
    });
    return { rows: nodes, wireMap: map, virtualOriginEnd, dividerRow };
  }, [commits.data, worktrees.data, hiddenWorktrees, mode, nowTs]);

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
  }, [params, rows, worktrees.data]);

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
  }, [params, focusSha, focusTick, rows, commits]);

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
      <div className="flex shrink-0 items-center justify-end gap-1.5 border-b border-border px-2 py-1 text-muted-foreground">
        <span className="text-[10px]" title="轻量模式：只保留分支/tag/merge/分叉点，隐藏其余提交">
          轻量拓扑
        </span>
        <Switch checked={mode === 'lite'} onCheckedChange={toggleMode} aria-label="轻量拓扑" />
      </div>
      <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto">
        <div className="relative">
          {/* 轻量模式「最近 N 个」下边界：overlay 画虚线，不占行高（不破坏行内 SVG 对齐） */}
          {dividerRow >= 0 ? (
            <div
              className="pointer-events-none absolute inset-x-0 z-10 border-t border-dashed border-muted-foreground/50"
              style={{ top: (dividerRow + 1) * ROW_H }}
            />
          ) : null}
          {rows.map((c, i) => (
            <CommitRow
              key={c.sha}
              c={c}
              laneWidth={laneWidth}
              wires={wireMap.get(i - 1) ?? []}
              virtualOriginEnd={virtualOriginEnd}
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
      </div>
    </>
  );
}

function CommitRow({
  c,
  laneWidth,
  wires,
  virtualOriginEnd,
  params,
  active,
  onAddWorktree,
}: {
  c: RowCommit;
  laneWidth: number;
  wires: GraphWire[];
  virtualOriginEnd: Map<number, number>;
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
          // 虚拟节点的连线段（线身份为虚拟节点行、且尚未到其 HEAD 行）置灰
          const end = virtualOriginEnd.get(w.origin);
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

// 泳道几何：列宽/行高/左边距；调色板与后端 color 索引对应（循环取色）
const LANE_W = 12;
const ROW_H = 28;
const LANE_X0 = 8;
const LANE_PALETTE = ['#3b82f6', '#f97316', '#10b981', '#ec4899', '#8b5cf6', '#eab308', '#14b8a6', '#ef4444'];
// 未提交虚拟节点与其连线：灰色，与已提交节点区分。
// 须用具体色值——SVG 的 fill/stroke 属性不解析 CSS 变量（hsl(var(--...)) 会失效，
// fill 落回黑、stroke 落回 none），选明暗主题下都可辨的中灰
const VIRTUAL_COLOR = '#9ca3af';
