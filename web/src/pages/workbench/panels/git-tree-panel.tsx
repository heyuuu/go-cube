import { GitBranch, Monitor } from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, type MouseEvent } from 'react';
import { useSearchParams } from 'react-router';

import { ErrorBanner } from '@/components/error-banner';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import {
  useWorkbenchCommits,
  useWorkbenchInfo,
  useWorkbenchRefs,
  useWorkbenchStatus,
  type GraphCommit,
  type GraphWire,
} from '@/queries/workbench';

import { sameSource, selectDiffSide, selectSource, type TreeSource, type WorkbenchParams } from '../params';

// git 树面板（提案 1011）：工作台默认入口，取代 SourceTree 的核心视图。
// 上段 = 工作副本状态区（worktree 分组，各自分支/ahead-behind/脏状态）；
// 下段 = commit 图（后端 active-lanes 算法产出泳道坐标，SVG 平行线拓扑 + 无限滚动）。
// 核心交互「选择」：单击 = 单选（source）；cmd/ctrl 单击 = 追加双选（left/right）。
// 全部选中态写 URL（params 模块统一管理），本面板只是 URL 的渲染者。

export function GitTreePanel({ params }: { params: WorkbenchParams }) {
  const { path } = params;
  const info = useWorkbenchInfo(path);

  if (info.isPending) {
    return <div className="p-3 text-xs text-muted-foreground">加载中…</div>;
  }
  if (info.isError) {
    return <ErrorBanner message={info.error.message} />;
  }

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <div className="shrink-0 overflow-y-auto border-b border-border">
        <WorktreeSection path={path} worktrees={info.data?.worktrees ?? []} params={params} />
      </div>
      <CommitGraphSection path={path} params={params} />
    </div>
  );
}

// --- 工作副本状态区 ---

function WorktreeSection({
  path,
  worktrees,
  params,
}: {
  path: string;
  worktrees: { path: string; branch: string; detached: boolean; bare: boolean }[];
  params: WorkbenchParams;
}) {
  const refs = useWorkbenchRefs(path);

  return (
    <>
      <Section title="工作副本" icon={<Monitor className="size-3.5" />}>
        {worktrees.map((wt) => (
          <WorktreeRow key={wt.path} path={path} wt={wt} params={params} />
        ))}
      </Section>
      <Section title="分支" icon={<GitBranch className="size-3.5" />}>
        {(refs.data?.locals ?? []).map((b) => (
          <SelectableRow
            key={b}
            label={b}
            source={{ type: 'ref', id: b }}
            params={params}
            badge={b === refs.data?.current ? '当前' : undefined}
          />
        ))}
      </Section>
    </>
  );
}

function WorktreeRow({
  path,
  wt,
  params,
}: {
  path: string;
  wt: { path: string; branch: string; detached: boolean; bare: boolean };
  params: WorkbenchParams;
}) {
  const status = useWorkbenchStatus(path, wt.path);
  const src: TreeSource = { type: 'worktree', id: wt.path };
  const name = wt.path.split('/').pop() || wt.path;

  return (
    <SelectableRow
      label={name}
      source={src}
      params={params}
      title={wt.path}
      badges={
        <>
          {wt.bare ? <Badge variant="outline">bare</Badge> : null}
          {status.data ? (
            <>
              {status.data.ahead > 0 ? <Badge variant="secondary">↑{status.data.ahead}</Badge> : null}
              {status.data.behind > 0 ? <Badge variant="secondary">↓{status.data.behind}</Badge> : null}
              {status.data.dirty ? (
                <Badge variant="destructive" className="px-1">
                  脏 {status.data.staged + status.data.unstaged + status.data.untracked}
                </Badge>
              ) : null}
            </>
          ) : wt.detached ? (
            <Badge variant="outline">detached</Badge>
          ) : null}
        </>
      }
    />
  );
}

// --- commit 图 ---

function CommitGraphSection({ path, params }: { path: string; params: WorkbenchParams }) {
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

  // 翻页边界去重（仓库有新提交时 skip 分页可能重复）；wires 以绝对行号为键累积
  const { rows, wireMap } = useMemo(() => {
    const seen = new Set<string>();
    const list: GraphCommit[] = [];
    const map = new Map<number, GraphWire[]>();
    for (const page of commits.data?.pages ?? []) {
      for (const c of page.list ?? []) {
        if (seen.has(c.sha)) continue;
        seen.add(c.sha);
        list.push(c);
      }
      for (const w of page.wires ?? []) {
        map.set(w.row, [...(map.get(w.row) ?? []), w]);
      }
    }
    return { rows: list, wireMap: map };
  }, [commits.data]);

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
    <div className="min-h-0 flex-1 overflow-y-auto">
      {rows.map((c, i) => (
        <CommitRow key={c.sha} c={c} laneWidth={laneWidth} wires={wireMap.get(i - 1) ?? []} params={params} />
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
  params,
}: {
  c: GraphCommit;
  laneWidth: number;
  wires: GraphWire[];
  params: WorkbenchParams;
}) {
  const nodeColor = LANE_PALETTE[(c.color ?? 0) % LANE_PALETTE.length];
  return (
    // 行高用固定 px（与 svg 的 ROW_H 同源）：根字号随视口 clamp 缩放，
    // rem 行高会与大屏下的 svg px 几何错位
    <div className="flex items-stretch" style={{ height: ROW_H }}>
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
          const color = LANE_PALETTE[w.color % LANE_PALETTE.length];
          return x1 === x2 ? (
            <line key={wi} x1={x1} y1={-ROW_H / 2} x2={x2} y2={ROW_H / 2} stroke={color} strokeWidth={1.5} />
          ) : (
            <path
              key={wi}
              d={`M ${x1} ${-ROW_H / 2} C ${x1} 0, ${x2} 0, ${x2} ${ROW_H / 2}`}
              fill="none"
              stroke={color}
              strokeWidth={1.5}
            />
          );
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
        label={c.subject}
        source={{ type: 'commit', id: c.sha }}
        params={params}
        title={`${c.sha}\n${c.author}\n${formatCommitTime(c.timestamp).full}`}
        mono={c.shortSha}
        time={c.timestamp}
        laneColor={nodeColor}
        fixedRow
        badges={
          <>
            {(c.refs ?? []).slice(0, 3).map((r: string) => (
              <Badge
                key={r}
                variant="secondary"
                className="max-w-24 truncate border"
                style={{ borderColor: nodeColor, color: nodeColor }}
              >
                {r}
              </Badge>
            ))}
          </>
        }
      />
    </div>
  );
}

// 泳道几何：列宽/行高/左边距；调色板与后端 color 索引对应（循环取色）
const LANE_W = 12;
const ROW_H = 28;
const LANE_X0 = 8;
const LANE_PALETTE = ['#3b82f6', '#f97316', '#10b981', '#ec4899', '#8b5cf6', '#eab308', '#14b8a6', '#ef4444'];

// --- 通用可选中行 ---

type SelectableRowProps = {
  label: string;
  source: TreeSource;
  params: WorkbenchParams;
  title?: string;
  mono?: string;
  badge?: string;
  badges?: React.ReactNode;
  time?: number;
  laneColor?: string; // 泳道色：选中行以分支色描边
  fixedRow?: boolean; // commit 图行：固定 px 高度（外层行 div 已定高），不用 rem 行高
};

function SelectableRow({
  label,
  source,
  params,
  title,
  mono,
  badge,
  badges,
  time,
  laneColor,
  fixedRow,
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
          }
          return next;
        },
        { replace: true },
      );
    },
    [source, setSearchParams],
  );

  const isSource = sameSource(params.source, source);
  const isLeft = sameSource(params.left, source);
  const isRight = sameSource(params.right, source);

  return (
    <button
      type="button"
      title={title}
      onClick={handleClick}
      className={cn(
        'min-w-0 flex-1 flex items-center gap-1.5 px-2 text-left text-xs transition-colors hover:bg-accent',
        fixedRow ? 'h-full' : 'leading-7',
        (isSource || isLeft || isRight) && 'bg-primary/15',
      )}
      style={laneColor && (isSource || isLeft || isRight) ? { boxShadow: `inset 2px 0 0 ${laneColor}` } : undefined}
    >
      {mono ? <span className="shrink-0 font-mono text-[10px] text-muted-foreground">{mono}</span> : null}
      <span className={cn('truncate', isSource || isLeft || isRight ? 'font-medium' : undefined)}>{label}</span>
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
