// Projects 页的展示单元格：可点徽标、git 状态格、最近使用时间、forge icon 组。
import type { ReactNode } from 'react';

import type { Project } from '@/api/client';
import { Badge } from '@/components/ui/badge';
import { renderIcon, type IconDecl } from '@/lib/icon';
import { formatDateTime, prettyTime } from '@/lib/time';

import type { GitStatus } from './filters';

export function ClickBadge({
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

export function GitCell({ p, onFilter }: { p: Project; onFilter: (s: GitStatus) => void }) {
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

// 最近使用时间：muted 等宽小字（刻意区别于 tag badge 的样式语言），悬停见绝对时间
export function LastUsedTime({ iso }: { iso: string }) {
  return (
    <span
      className="shrink-0 font-mono text-[0.6875rem] text-muted-foreground/70"
      title={`最近使用：${formatDateTime(iso)}`}
    >
      {prettyTime(iso)}
    </span>
  );
}

// 项目 forge 图标：remote host 匹配到已配置 forge 时展示其 icon（1040）；
// 多 remote 项目可同时展示多个（1042 修）；未配置 forge 或 forge 未配 icon 时不展示（无兜底图标）
export function ForgeIcon({ matches }: { matches: { host: string; icon: IconDecl }[] }) {
  if (matches.length === 0) return null;
  return (
    <span className="flex shrink-0 items-center gap-0.5 text-muted-foreground">
      {matches.map((m) => (
        <span key={m.host} title={`forge：${m.host}`}>
          {renderIcon(m.icon, null)}
        </span>
      ))}
    </span>
  );
}

