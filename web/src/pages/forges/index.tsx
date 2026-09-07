// forge 页（提案 1042，模型 1044）：以 account 拉取的远端仓库为入口的列表视图（无 tree 模式）。
// 数据源 /api/forge/overview（各 account 拉取缓存 × 本地项目快照的聚合对账，纯读不外呼）；
// 拉取/刷新是显式动作（account 摘要条的刷新按钮）；owner 从 URL 推导、仅作展示分组维度。
import { CloudDownload, Copy, ExternalLink, RefreshCw } from 'lucide-react';
import { useState } from 'react';
import { Link, useSearchParams } from 'react-router';

import type { components, Forge } from '@/api/client';
import { EmptyState } from '@/components/empty-state';
import { ErrorBanner } from '@/components/error-banner';
import { Chip, FilterRow, SortHead } from '@/components/filter-chips';
import { PageHeader } from '@/components/page-header';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { repoPathOf } from '@/lib/forge';
import type { IconDecl } from '@/lib/icon';
import { renderIcon } from '@/lib/icon';
import { guessHome, prettyPath } from '@/lib/path';
import { prettyTime } from '@/lib/time';
import { cn } from '@/lib/utils';
import { useForgeAccountFetch, useForgeOverview, useForges } from '@/queries/forge';

type RepoRow = components['schemas']['RepoRow'];

// 对账状态筛选（URL ?status= 记忆）
const STATUS_FILTERS = [
  { value: 'all', label: '全部' },
  { value: 'missing', label: '未 clone' },
  { value: 'synced', label: '已 clone' },
  { value: 'orphan', label: '孤儿' },
] as const;

const STATUS_META: Record<string, { label: string; className: string }> = {
  missing: { label: '未 clone', className: 'text-sky-600' },
  synced: { label: '已 clone', className: 'text-emerald-600' },
  orphan: { label: '孤儿', className: 'text-amber-600' },
};

// 排序键（URL ?sort= 记忆）：默认按更新时间倒序（远端仓库时间）
type SortMode = 'updated' | 'updated-asc' | 'name' | 'name-desc';

function isSortMode(v: string | null): v is SortMode {
  return v === 'updated' || v === 'updated-asc' || v === 'name' || v === 'name-desc';
}

// fetchedAt 超过该天数视为「数据已过期」（页头徽标提示，不自动外呼）
const STALE_DAYS = 7;

function isStale(iso: string): boolean {
  if (!iso) return false;
  return Date.now() - new Date(iso).getTime() > STALE_DAYS * 86400_000;
}

function sortRows(rows: RepoRow[], mode: SortMode): RepoRow[] {
  const sorted = [...rows];
  const byName = (a: RepoRow, b: RepoRow) => rowDisplay(a).text.localeCompare(rowDisplay(b).text);
  switch (mode) {
    case 'name':
      sorted.sort(byName);
      break;
    case 'name-desc':
      sorted.sort((a, b) => byName(b, a));
      break;
    case 'updated-asc':
      sorted.sort((a, b) => new Date(a.repo.updatedAt ?? 0).getTime() - new Date(b.repo.updatedAt ?? 0).getTime());
      break;
    default:
      sorted.sort((a, b) => new Date(b.repo.updatedAt ?? 0).getTime() - new Date(a.repo.updatedAt ?? 0).getTime());
  }
  return sorted;
}

// RepoUrl 列展示文本：owner/name（host 由 forge icon 承载）；孤儿行取本地 remote 的 repo url
function rowDisplay(r: RepoRow): { text: string; title: string } {
  if (r.status === 'orphan' && r.local) {
    return { text: repoPathOf(r.local.repoUrl), title: r.local.repoUrl };
  }
  const text = r.repo.fullName || repoPathOf(r.repo.cloneUrl);
  return { text, title: r.repo.cloneUrl };
}

export function ForgesPage() {
  const overview = useForgeOverview();
  const forgesQ = useForges();
  const fetchMut = useForgeAccountFetch();
  const [params, setParams] = useSearchParams();
  const [copied, setCopied] = useState('');

  const forges: Forge[] = forgesQ.data?.list ?? [];
  const forgeIcon = (host: string): IconDecl | undefined => {
    const f = forges.find((x) => x.host === host);
    return f?.icon ? { type: f.icon.type, value: f.icon.value } : undefined;
  };

  const forgeFilter = params.get('forge') ?? 'all';
  const statusFilter = params.get('status') ?? 'all';
  const sortParam = params.get('sort');
  const sortMode: SortMode = isSortMode(sortParam) ? sortParam : 'updated';
  const setParam = (key: string, value: string | null) => {
    const next = new URLSearchParams(params);
    if (value === null) next.delete(key);
    else next.set(key, value);
    setParams(next, { replace: true });
  };

  // 路径展示用 ~ 缩写（与项目页一致）；home 从本地行推导
  const home = guessHome((overview.data?.rows ?? []).map((r) => r.local?.path ?? '').filter(Boolean));

  const accounts = overview.data?.accounts ?? [];
  const fetchedCount = accounts.filter((a) => !a.fetchedAt?.startsWith('0001')).length;

  const rows = sortRows(
    (overview.data?.rows ?? []).filter(
      (r) =>
        (forgeFilter === 'all' || r.forgeHost === forgeFilter) && (statusFilter === 'all' || r.status === statusFilter),
    ),
    sortMode,
  );

  const copy = (label: string, text: string) => {
    void navigator.clipboard.writeText(text);
    setCopied(label);
    window.setTimeout(() => setCopied(''), 1500);
  };

  return (
    <div className="pb-10">
      <PageHeader
        title="Forges"
        meta={<span>远端仓库视角（对账缓存，不实时外呼）；clone / 推拉等 git 操作仍走本机凭证与既有流程</span>}
      />

      {/* account 摘要条：拉取新鲜度 + 显式刷新入口 */}
      <div className="mx-6 mb-3 flex flex-wrap items-center gap-2">
        {accounts.map((a) => {
          const never = !a.fetchedAt || a.fetchedAt.startsWith('0001');
          const stale = !never && isStale(a.fetchedAt!);
          return (
            <span
              key={`${a.forgeHost}/${a.username}`}
              className={cn(
                'flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs',
                (never || stale) && 'border-amber-500/50 bg-amber-500/5',
              )}
            >
              {renderIcon(forgeIcon(a.forgeHost), null)}
              <span className="font-mono">{a.username}</span>
              {never ? (
                <Badge variant="outline" className="text-amber-600">
                  未拉取
                </Badge>
              ) : (
                <>
                  <span className="text-muted-foreground">
                    {a.repoCount} 仓库 · 拉取于 {prettyTime(a.fetchedAt)}
                  </span>
                  {stale && (
                    <Badge variant="outline" className="text-amber-600">
                      已过期
                    </Badge>
                  )}
                </>
              )}
              <button
                type="button"
                aria-label={`刷新 ${a.username}@${a.forgeHost}`}
                className="text-muted-foreground hover:text-foreground"
                disabled={fetchMut.isPending}
                onClick={() =>
                  fetchMut.mutate(
                    { forgeHost: a.forgeHost, username: a.username, force: true },
                    { onSuccess: () => void overview.refetch() },
                  )
                }
              >
                <RefreshCw className={cn('size-3', fetchMut.isPending && 'animate-spin')} />
              </button>
            </span>
          );
        })}
        {accounts.length === 0 && (
          <span className="text-xs text-muted-foreground">
            暂无账号，先到
            <Link to="/settings?section=forge" target="_blank" className="mx-1 underline">
              设置 · Forge
            </Link>
            配置（不配 token 无法拉取）
          </span>
        )}
        {accounts.length > 0 && fetchedCount < accounts.length && (
          <span className="text-xs text-amber-600">
            {accounts.length - fetchedCount} 个账号未拉取，不出现在下方列表
          </span>
        )}
      </div>

      {/* 筛选 chips（每维一行，标签标注单选/多选，同 Projects 页） */}
      <div className="mx-6 mb-3 flex flex-col gap-1.5 text-xs">
        <FilterRow label="forge" mode="单选">
          <Chip active={forgeFilter === 'all'} onClick={() => setParam('forge', null)}>
            全部
          </Chip>
          {forges.map((f) => (
            <Chip key={f.host} active={forgeFilter === f.host} onClick={() => setParam('forge', f.host)}>
              <span className="flex items-center gap-1">
                {renderIcon(f.icon ? { type: f.icon.type, value: f.icon.value } : undefined, null)}
                {f.host}
              </span>
            </Chip>
          ))}
        </FilterRow>
        <FilterRow label="状态" mode="单选">
          {STATUS_FILTERS.map((s) => (
            <Chip
              key={s.value}
              active={statusFilter === s.value}
              onClick={() => setParam('status', s.value === 'all' ? null : s.value)}
            >
              {s.label}
            </Chip>
          ))}
        </FilterRow>
      </div>

      {overview.error && <ErrorBanner message={`加载失败：${overview.error.message}`} />}
      {fetchMut.error && <ErrorBanner message={`拉取失败：${fetchMut.error.message}`} />}

      <div className="mx-6 rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <SortHead
                label="RepoUrl"
                state={sortMode === 'name' ? 'asc' : sortMode === 'name-desc' ? 'desc' : null}
                onCycle={() =>
                  setParam('sort', sortMode === 'name' ? 'name-desc' : sortMode === 'name-desc' ? null : 'name')
                }
              />
              <TableHead>状态</TableHead>
              <TableHead>本地</TableHead>
              <SortHead
                label="更新时间"
                state={sortMode === 'updated' ? 'desc' : sortMode === 'updated-asc' ? 'asc' : null}
                onCycle={() =>
                  setParam(
                    'sort',
                    sortMode === 'updated' ? 'updated-asc' : sortMode === 'updated-asc' ? null : 'updated',
                  )
                }
              />
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} className="text-xs text-muted-foreground">
                  {overview.isLoading ? '加载中…' : '暂无仓库行——先在上方刷新账号拉取'}
                </TableCell>
              </TableRow>
            )}
            {rows.map((r) => {
              const status = STATUS_META[r.status];
              const display = rowDisplay(r);
              const cloneCmd = `cube clone ${r.repo.cloneUrl}`;
              return (
                <TableRow key={`${r.forgeHost}/${r.owner}/${display.text}/${r.status}`}>
                  <TableCell className="font-medium">
                    <span className="flex items-center gap-1.5">
                      {renderIcon(forgeIcon(r.forgeHost), null)}
                      <span title={display.title}>{display.text || '—'}</span>
                      {r.repo.defaultBranch && r.status !== 'orphan' && (
                        <span className="font-mono text-[10px] text-muted-foreground">{r.repo.defaultBranch}</span>
                      )}
                    </span>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className={status?.className}>
                      {status?.label ?? r.status}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {r.local ? (
                      <span className="flex items-center gap-1.5 text-xs">
                        <span className="max-w-48 truncate font-mono text-muted-foreground" title={r.local.path}>
                          {prettyPath(r.local.path, home)}
                        </span>
                        {r.local.dirty && (
                          <Badge variant="outline" className="text-amber-600">
                            dirty
                          </Badge>
                        )}
                        {r.local.ahead > 0 && <Badge variant="outline">↑{r.local.ahead}</Badge>}
                        {r.local.behind > 0 && <Badge variant="outline">↓{r.local.behind}</Badge>}
                      </span>
                    ) : (
                      <span className="text-xs text-muted-foreground">—</span>
                    )}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">{prettyTime(r.repo.updatedAt)}</TableCell>
                  <TableCell className="text-right">
                    {r.status === 'missing' && (
                      <Button size="sm" variant="ghost" onClick={() => copy(cloneCmd, cloneCmd)}>
                        <CloudDownload className="size-3.5" />
                        复制 clone 命令
                      </Button>
                    )}
                    {r.status === 'synced' && r.local && (
                      <span className="flex items-center justify-end gap-1">
                        <Button size="sm" variant="ghost" onClick={() => copy(r.local!.name, r.local!.path)}>
                          <Copy className="size-3.5" />
                          路径
                        </Button>
                        <Link to={`/workbench?path=${encodeURIComponent(r.local.path)}`} target="_blank">
                          <Button size="sm" variant="ghost">
                            <ExternalLink className="size-3.5" />
                            工作台
                          </Button>
                        </Link>
                      </span>
                    )}
                    {r.status === 'orphan' && r.local && (
                      <Button size="sm" variant="ghost" onClick={() => copy(r.local!.name, r.local!.path)}>
                        <Copy className="size-3.5" />
                        复制路径
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>
      {rows.length > 0 && (
        <div className="mx-6 mt-2 text-xs text-muted-foreground">
          {rows.length} 个仓库{copied && <span className="ml-3 text-emerald-600">已复制：{copied}</span>}
        </div>
      )}
      {!overview.isLoading && (overview.data?.rows ?? []).length === 0 && accounts.length > 0 && (
        <EmptyState title="还没有对账数据" sub="点击上方账号摘要的 ↻ 拉取远端仓库列表" />
      )}
    </div>
  );
}
