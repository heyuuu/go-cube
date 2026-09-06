// forge 页（提案 1042）：以 forge 上的远端仓库为入口的列表视图（无 tree 模式）。
// 数据源 /api/forge/overview（1041 拉取缓存 × 本地项目快照的聚合对账，纯读不外呼）；
// 拉取/刷新是显式动作（namespace 摘要条的刷新按钮），行内「复制 clone 命令」不做 clone 执行。
import { CloudDownload, Copy, ExternalLink, RefreshCw } from 'lucide-react';
import { useState } from 'react';
import { Link, useSearchParams } from 'react-router';

import type { components, Forge } from '@/api/client';

type RepoRow = components['schemas']['RepoRow'];
import { EmptyState } from '@/components/empty-state';
import { ErrorBanner } from '@/components/error-banner';
import { PageHeader } from '@/components/page-header';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import type { IconDecl } from '@/lib/icon';
import { renderIcon } from '@/lib/icon';
import { prettyTime } from '@/lib/time';
import { cn } from '@/lib/utils';
import { useForgeNamespaceFetch, useForgeOverview, useForges } from '@/queries/forge';

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

// fetchedAt 超过该天数视为「数据已过期」（页头徽标提示，不自动外呼）
const STALE_DAYS = 7;

function isStale(iso: string): boolean {
  if (!iso) return false;
  return Date.now() - new Date(iso).getTime() > STALE_DAYS * 86400_000;
}

// 行排序：更新时间倒序（远端仓库时间）为默认，可切名称；零值时间（orphan 行）排最后
function sortRows(rows: RepoRow[], mode: 'updated' | 'name'): RepoRow[] {
  const sorted = [...rows];
  if (mode === 'name') {
    sorted.sort((a, b) => rowName(a).localeCompare(rowName(b)));
  } else {
    sorted.sort((a, b) => new Date(b.repo.updatedAt ?? 0).getTime() - new Date(a.repo.updatedAt ?? 0).getTime());
  }
  return sorted;
}

function rowName(r: RepoRow): string {
  return r.local?.name || r.repo.name || '';
}

export function ForgesPage() {
  const overview = useForgeOverview();
  const forgesQ = useForges();
  const fetchMut = useForgeNamespaceFetch();
  const [params, setParams] = useSearchParams();
  const [copied, setCopied] = useState('');

  const forges: Forge[] = forgesQ.data?.list ?? [];
  const forgeIcon = (host: string): IconDecl | undefined => {
    const f = forges.find((x) => x.host === host);
    return f?.icon ? { type: f.icon.type, value: f.icon.value } : undefined;
  };

  const forgeFilter = params.get('forge') ?? 'all';
  const statusFilter = params.get('status') ?? 'all';
  const sort = (params.get('sort') ?? 'updated') as 'updated' | 'name';
  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(params);
    if (value === 'all' || value === 'updated') next.delete(key);
    else next.set(key, value);
    setParams(next, { replace: true });
  };

  const namespaces = overview.data?.namespaces ?? [];
  const fetchedCount = namespaces.filter((ns) => !ns.fetchedAt?.startsWith('0001')).length;

  const rows = sortRows(
    (overview.data?.rows ?? []).filter(
      (r) =>
        (forgeFilter === 'all' || r.forgeHost === forgeFilter) && (statusFilter === 'all' || r.status === statusFilter),
    ),
    sort,
  );

  const copy = (label: string, text: string) => {
    void navigator.clipboard.writeText(text);
    setCopied(label);
    window.setTimeout(() => setCopied(''), 1500);
  };

  return (
    <div className="pb-10">
      <PageHeader
        title="Forge"
        meta={<span>远端仓库视角（对账缓存，不实时外呼）；clone / 推拉等 git 操作仍走本机凭证与既有流程</span>}
      />

      {/* namespace 摘要条：拉取新鲜度 + 显式刷新入口 */}
      <div className="mx-6 mb-3 flex flex-wrap items-center gap-2">
        {namespaces.map((ns) => {
          const never = !ns.fetchedAt || ns.fetchedAt.startsWith('0001');
          const stale = !never && isStale(ns.fetchedAt!);
          return (
            <span
              key={`${ns.forgeHost}/${ns.path}`}
              className={cn(
                'flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs',
                (never || stale) && 'border-amber-500/50 bg-amber-500/5',
              )}
            >
              {renderIcon(forgeIcon(ns.forgeHost), null)}
              <span className="font-mono">{ns.path}</span>
              {never ? (
                <Badge variant="outline" className="text-amber-600">
                  未拉取
                </Badge>
              ) : (
                <>
                  <span className="text-muted-foreground">
                    {ns.repoCount} 仓库 · 拉取于 {prettyTime(ns.fetchedAt)}
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
                aria-label={`刷新 ${ns.path}`}
                className="text-muted-foreground hover:text-foreground"
                disabled={fetchMut.isPending}
                onClick={() =>
                  fetchMut.mutate(
                    { forgeHost: ns.forgeHost, path: ns.path, force: true },
                    { onSuccess: () => void overview.refetch() },
                  )
                }
              >
                <RefreshCw className={cn('size-3', fetchMut.isPending && 'animate-spin')} />
              </button>
            </span>
          );
        })}
        {namespaces.length === 0 && (
          <span className="text-xs text-muted-foreground">
            暂无 namespace，先到
            <Link to="/settings?section=forge" target="_blank" className="mx-1 underline">
              设置 · Forge
            </Link>
            配置
          </span>
        )}
        {namespaces.length > 0 && fetchedCount < namespaces.length && (
          <span className="text-xs text-amber-600">
            {namespaces.length - fetchedCount} 个 namespace 未拉取，不出现在下方列表
          </span>
        )}
      </div>

      {/* 筛选 chips：forge / 对账状态 / 排序（URL 记忆） */}
      <div className="mx-6 mb-2 flex flex-wrap items-center gap-x-4 gap-y-1.5">
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-muted-foreground">forge</span>
          {[{ host: 'all', label: '全部' }, ...forges.map((f) => ({ host: f.host, label: f.host }))].map((f) => (
            <button
              key={f.host}
              type="button"
              onClick={() => setParam('forge', f.host)}
              className={chipClass(forgeFilter === f.host)}
            >
              {f.host !== 'all' && renderIcon(forgeIcon(f.host), null)}
              {f.label}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-muted-foreground">状态</span>
          {STATUS_FILTERS.map((s) => (
            <button
              key={s.value}
              type="button"
              onClick={() => setParam('status', s.value)}
              className={chipClass(statusFilter === s.value)}
            >
              {s.label}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-muted-foreground">排序</span>
          <button type="button" onClick={() => setParam('sort', 'updated')} className={chipClass(sort === 'updated')}>
            更新时间
          </button>
          <button type="button" onClick={() => setParam('sort', 'name')} className={chipClass(sort === 'name')}>
            名称
          </button>
        </div>
      </div>

      {overview.error && <ErrorBanner message={`加载失败：${overview.error.message}`} />}
      {fetchMut.error && <ErrorBanner message={`拉取失败：${fetchMut.error.message}`} />}

      <div className="mx-6 rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>仓库</TableHead>
              <TableHead>forge</TableHead>
              <TableHead>namespace</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>本地</TableHead>
              <TableHead>更新时间</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={7} className="text-xs text-muted-foreground">
                  {overview.isLoading ? '加载中…' : '暂无仓库行——先在上方刷新 namespace 拉取'}
                </TableCell>
              </TableRow>
            )}
            {rows.map((r) => {
              const status = STATUS_META[r.status];
              const cloneCmd = `cube clone ${r.repo.cloneUrl}`;
              return (
                <TableRow key={`${r.forgeHost}/${r.nsPath}/${rowName(r)}/${r.status}`}>
                  <TableCell className="font-medium">
                    <span className="flex items-center gap-1.5">
                      {r.repo.name}
                      {r.repo.defaultBranch && (
                        <span className="font-mono text-[10px] text-muted-foreground">{r.repo.defaultBranch}</span>
                      )}
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className="flex items-center gap-1 font-mono text-xs text-muted-foreground">
                      {renderIcon(forgeIcon(r.forgeHost), null)}
                      {r.forgeHost}
                    </span>
                  </TableCell>
                  <TableCell className="font-mono text-xs">{r.nsPath}</TableCell>
                  <TableCell>
                    <Badge variant="outline" className={status?.className}>
                      {status?.label ?? r.status}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {r.local ? (
                      <span className="flex items-center gap-1.5 text-xs">
                        <span className="max-w-48 truncate font-mono text-muted-foreground" title={r.local.path}>
                          {r.local.path}
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
      {!overview.isLoading && (overview.data?.rows ?? []).length === 0 && namespaces.length > 0 && (
        <EmptyState title="还没有对账数据" sub="点击上方 namespace 摘要的 ↻ 拉取远端仓库列表" />
      )}
    </div>
  );
}

function chipClass(active: boolean): string {
  return cn(
    'flex items-center gap-1 rounded-full border px-2.5 py-0.5 font-mono text-xs transition-colors',
    active
      ? 'border-primary bg-primary text-primary-foreground'
      : 'text-muted-foreground hover:bg-muted hover:text-foreground',
  );
}
