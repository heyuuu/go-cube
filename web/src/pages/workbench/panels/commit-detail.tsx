import { useLocalPref } from '@/hooks/use-local-pref';

import { ErrorBanner } from '@/components/error-banner';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { useWorkbenchCommit } from '@/queries/workbench';

import type { TreeSource } from '../params';
import { RowSplitter } from '../splitter';

import { formatCommitTime, REF_BADGE_STYLE } from './commit-bits';

// 提交详情区（内容面板目录树下方的 split 区）：当前源为 ref/commit 时展示指向提交的
// 完整信息。解决 AI 时代 commit 信息长且多行、commit 图行内展示不全的问题。
// 表单式布局（标签列 + 值列），message 正文整段换行展示、可滚动；
// 上方目录树与详情区的分界线可拖拽调高度（持久化）。

const HEIGHT_KEY = 'cube.workbench.commitdetail.height';
const MIN_HEIGHT = 80;
const MAX_HEIGHT = 480;

export function CommitDetailPane({ path, source }: { path: string; source: TreeSource }) {
  const commit = useWorkbenchCommit(path, source);
  const [height, setHeight] = useLocalPref<number>(HEIGHT_KEY, 200, (raw) => {
    const v = Number(raw);
    return Number.isFinite(v) && v >= MIN_HEIGHT && v <= MAX_HEIGHT ? v : 200;
  });

  if (commit.isError) {
    return (
      <>
        <RowSplitter onDelta={() => {}} />
        <div className="shrink-0" style={{ height }}>
          <ErrorBanner message={commit.error.message} />
        </div>
      </>
    );
  }
  const d = commit.data;
  return (
    <>
      {/* 分隔条上拖 = 详情区变高（dy 为负） */}
      <RowSplitter onDelta={(dy) => setHeight((h) => Math.min(MAX_HEIGHT, Math.max(MIN_HEIGHT, h - dy)))} />
      <div className="flex shrink-0 flex-col overflow-hidden" style={{ height }}>
        {d ? (
          <div className="min-h-0 flex-1 overflow-y-auto px-2 py-1.5">
            <dl className="flex flex-col gap-1 text-xs">
              <DetailRow label="commit">
                <span className="font-mono text-[11px]" title={d.sha}>
                  {d.sha}
                </span>
              </DetailRow>
              <DetailRow label="作者">
                <span className="truncate">{d.author}</span>
              </DetailRow>
              <DetailRow label="时间">
                <span title={`unix ${d.timestamp}`}>{formatCommitTime(d.timestamp).full}</span>
              </DetailRow>
              {d.refs && d.refs.length > 0 ? (
                <DetailRow label="引用">
                  <div className="flex flex-wrap gap-1">
                    {d.refs.map((r) => (
                      <Badge
                        key={r.kind + r.name}
                        variant="outline"
                        className={cn(
                          'max-w-40 truncate border px-1 py-0 text-[10px]',
                          REF_BADGE_STYLE[r.kind as keyof typeof REF_BADGE_STYLE] ?? REF_BADGE_STYLE.local,
                        )}
                      >
                        {r.name}
                      </Badge>
                    ))}
                  </div>
                </DetailRow>
              ) : null}
              {d.parents && d.parents.length > 1 ? (
                <DetailRow label="父提交">
                  <div className="flex flex-col font-mono text-[11px] break-all">{d.parents.join('\n')}</div>
                </DetailRow>
              ) : null}
              <DetailRow label="标题" block>
                <p className="leading-5 font-medium break-words">{d.subject}</p>
              </DetailRow>
              {d.body ? (
                <DetailRow label="正文" block>
                  <pre className="font-mono text-[11px] leading-5 whitespace-pre-wrap break-words text-foreground/90">
                    {d.body}
                  </pre>
                </DetailRow>
              ) : null}
            </dl>
          </div>
        ) : (
          <div className="px-2 py-1 text-[10px] text-muted-foreground">提交信息加载中…</div>
        )}
      </div>
    </>
  );
}

// 表单行：block = 值独占后续行（标题/正文等多行内容），否则标签列 + 值同行
function DetailRow({ label, block, children }: { label: string; block?: boolean; children: React.ReactNode }) {
  return (
    <div className={block ? 'flex flex-col gap-0.5' : 'flex gap-2'}>
      <dt className="shrink-0 text-[10px] leading-5 text-muted-foreground">{block ? label : `${label}：`}</dt>
      <dd className={block ? '' : 'min-w-0 flex-1'}>{children}</dd>
    </div>
  );
}
