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
  type CommitEntry,
} from '@/queries/workbench';

import { sameSource, selectDiffSide, selectSource, type TreeSource, type WorkbenchParams } from '../params';

// git 树面板（提案 1011）：工作台默认入口，取代 SourceTree 的核心视图。
// 上段 = 工作副本状态区（worktree 分组，各自分支/ahead-behind/脏状态）；
// 下段 = commit 图（简化拓扑 + 无限滚动）。
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

  // 翻页边界去重（仓库有新提交时 skip 分页可能重复）
  const rows = useMemo(() => {
    const seen = new Set<string>();
    const list: CommitEntry[] = [];
    for (const page of commits.data?.pages ?? []) {
      for (const c of page.list ?? []) {
        if (seen.has(c.sha)) continue;
        seen.add(c.sha);
        list.push(c);
      }
    }
    return list;
  }, [commits.data]);

  if (commits.isError) {
    return (
      <div className="min-h-0 flex-1 overflow-hidden">
        <ErrorBanner message={commits.error.message} />
      </div>
    );
  }

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      {rows.map((c) => (
        <CommitRow key={c.sha} c={c} params={params} />
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

function CommitRow({ c, params }: { c: CommitEntry; params: WorkbenchParams }) {
  // 简化拓扑：多父提交 = 合并点标记；完整平行线拓扑待使用反馈再升级
  const isMerge = (c.parents ?? []).length > 1;
  return (
    <SelectableRow
      key={c.sha}
      label={c.subject}
      source={{ type: 'commit', id: c.sha }}
      params={params}
      title={`${c.sha}\n${c.author}`}
      mono={c.shortSha}
      dot={isMerge ? 'merge' : 'normal'}
      time={c.timestamp}
      badges={
        <>
          {(c.refs ?? []).slice(0, 3).map((r: string) => (
            <Badge key={r} variant="secondary" className="max-w-24 truncate">
              {r}
            </Badge>
          ))}
        </>
      }
    />
  );
}

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
  dot?: 'normal' | 'merge';
};

function SelectableRow({ label, source, params, title, mono, badge, badges, time, dot }: SelectableRowProps) {
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
        'flex w-full items-center gap-1.5 rounded-md px-2 py-1 text-left text-xs transition-colors hover:bg-accent',
        (isSource || isLeft || isRight) && 'bg-primary/15',
      )}
    >
      {dot ? (
        <span
          className={cn('size-2 shrink-0 rounded-full', dot === 'merge' ? 'bg-primary' : 'bg-muted-foreground/60')}
        />
      ) : null}
      {mono ? <span className="shrink-0 font-mono text-[10px] text-muted-foreground">{mono}</span> : null}
      <span className={cn('truncate', isSource || isLeft || isRight ? 'font-medium' : undefined)}>{label}</span>
      {badge ? <Badge variant="secondary">{badge}</Badge> : null}
      {badges}
      {isLeft ? <Badge className="ml-auto shrink-0">左</Badge> : null}
      {isRight ? <Badge className="ml-auto shrink-0">右</Badge> : null}
      {time ? <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">{relativeTime(time)}</span> : null}
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

function relativeTime(ts: number): string {
  const diff = Date.now() / 1000 - ts;
  if (diff < 60) return '刚刚';
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`;
  if (diff < 86400 * 30) return `${Math.floor(diff / 86400)}天前`;
  return new Date(ts * 1000).toLocaleDateString();
}
