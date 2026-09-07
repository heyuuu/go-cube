// git 树面板的通用小件：面板小节容器 + 可选中行（工作副本区 / 分支行 / commit 行共用）。
import { useCallback, type MouseEvent } from 'react';
import { useSearchParams } from 'react-router';

import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

import {
  selectCurrent,
  selectDiffSide,
  sameSource,
  type TreeSource,
  type WorkbenchParams,
} from '../params';

import { formatCommitTime } from './commit-bits';

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

export function SelectableRow({
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

export function Section({
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
