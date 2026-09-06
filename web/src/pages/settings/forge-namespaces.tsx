// settings 页 Forge 分区「命名空间」子表：namespace（仓库归属空间）的增删改、
// type 自动探测、远端拉取与对账展示（提案 1041）。对账结果走抽屉最小可用版。
import { useEffect, useState } from 'react';

import type { Forge, ForgeNamespace, ReconcileResult } from '@/api/client';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { ErrorBanner } from '@/components/error-banner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';
import {
  fetchNamespaceReconcile,
  useForgeAccounts,
  useForgeNamespaceDelete,
  useForgeNamespaceDetect,
  useForgeNamespaceFetch,
  useForgeNamespaceSave,
  useForgeNamespaces,
  useForges,
} from '@/queries/forge';

interface NamespaceDraft {
  forgeHost: string;
  path: string;
  type: string;
  accountUsername: string;
}

const EMPTY_NS_DRAFT: NamespaceDraft = { forgeHost: '', path: '', type: 'personal', accountUsername: '' };

// type chip 双值 + 自动识别入口
const NS_TYPES = [
  { value: 'personal', label: 'personal', hint: '个人空间' },
  { value: 'org', label: 'org', hint: '组织空间' },
];

function chipClass(active: boolean) {
  return cn(
    'rounded-full border px-2.5 py-0.5 font-mono text-xs transition-colors',
    active
      ? 'border-primary bg-primary text-primary-foreground'
      : 'text-muted-foreground hover:bg-muted hover:text-foreground',
  );
}

function NamespaceForm({
  draft,
  forges,
  accountUsernames,
  onClose,
}: {
  draft: NamespaceDraft;
  forges: Forge[];
  accountUsernames: string[];
  onClose: () => void;
}) {
  const [form, setForm] = useState<NamespaceDraft>(draft);
  const save = useForgeNamespaceSave();
  const detect = useForgeNamespaceDetect();
  const set = <K extends keyof NamespaceDraft>(key: K, value: NamespaceDraft[K]) =>
    setForm((f) => ({ ...f, [key]: value }));
  const isEdit = draft.path !== '';
  const apiForges = forges.filter((f) => f.kind !== 'generic');

  // 自动识别：调平台探测端点回填 type，可手动覆盖
  const runDetect = () => {
    detect.mutate(
      { forgeHost: form.forgeHost, path: form.path.trim() },
      {
        onSuccess: (data) => {
          const type = (data as { type?: string } | null)?.type;
          if (type) set('type', type);
        },
      },
    );
  };

  const submit = () => {
    save.mutate(
      {
        forgeHost: form.forgeHost,
        path: form.path.trim(),
        type: form.type,
        accountUsername: form.accountUsername,
      },
      { onSuccess: onClose },
    );
  };

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full gap-0 overflow-y-auto sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{isEdit ? '编辑命名空间' : '新增命名空间'}</SheetTitle>
          <SheetDescription>命名空间 = 平台上仓库归属的 path 前缀（个人空间或组织空间）</SheetDescription>
        </SheetHeader>
        <div className="flex flex-col gap-4 p-4 text-sm">
          {save.error && <ErrorBanner message={`保存失败：${save.error.message}`} />}

          <div className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">所属 forge</span>
            <div className="flex flex-wrap gap-1.5">
              {apiForges.map((f) => (
                <button
                  key={f.host}
                  type="button"
                  onClick={() => set('forgeHost', f.host)}
                  className={chipClass(form.forgeHost === f.host)}
                >
                  {f.host}
                </button>
              ))}
            </div>
          </div>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">path（命名空间路径，唯一键之一）</span>
            <Input
              value={form.path}
              onChange={(e) => set('path', e.target.value)}
              placeholder="heyuuu"
              className="font-mono"
            />
          </label>

          <div className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">type（决定拉取端点，可自动识别）</span>
            <div className="flex items-center gap-1.5">
              {NS_TYPES.map((t) => (
                <button
                  key={t.value}
                  type="button"
                  title={t.hint}
                  onClick={() => set('type', t.value)}
                  className={chipClass(form.type === t.value)}
                >
                  {t.label}
                </button>
              ))}
              <Button
                size="sm"
                variant="ghost"
                className="ml-auto"
                disabled={!form.forgeHost || !form.path.trim() || detect.isPending}
                onClick={runDetect}
              >
                {detect.isPending ? '识别中…' : '自动识别'}
              </Button>
            </div>
            {detect.error && <ErrorBanner message={`识别失败：${detect.error.message}`} />}
          </div>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">挂载账号（可选，拉取私有库用）</span>
            <div className="flex flex-wrap gap-1.5">
              <button
                type="button"
                onClick={() => set('accountUsername', '')}
                className={chipClass(form.accountUsername === '')}
              >
                不挂载（仅公开数据）
              </button>
              {accountUsernames.map((u) => (
                <button
                  key={u}
                  type="button"
                  onClick={() => set('accountUsername', u)}
                  className={chipClass(form.accountUsername === u)}
                >
                  {u}
                </button>
              ))}
            </div>
          </label>

          <div className="mt-2 flex justify-end gap-2">
            <Button size="sm" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button size="sm" disabled={save.isPending || !form.forgeHost || !form.path.trim()} onClick={submit}>
              保存
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}

// 对账结果抽屉：三类分区，最小可用展示
function ReconcileSheet({ ns, onClose }: { ns: ForgeNamespace; onClose: () => void }) {
  const [result, setResult] = useState<ReconcileResult | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    setError('');
    fetchNamespaceReconcile(ns.forgeHost, ns.path)
      .then((data) => setResult(data ?? null))
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [ns.forgeHost, ns.path]);

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full gap-0 overflow-y-auto sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>
            对账：{ns.path}@{ns.forgeHost}
          </SheetTitle>
          <SheetDescription>远端仓库列表（上次拉取缓存）与本地项目按 clone 地址匹配</SheetDescription>
        </SheetHeader>
        <div className="flex flex-col gap-4 p-4 text-sm">
          {error && <ErrorBanner message={error} />}
          {loading && <span className="text-xs text-muted-foreground">对账中…</span>}
          {!loading && result && (
            <>
              <div>
                <div className="mb-1 flex items-baseline gap-2">
                  <span className="text-xs font-medium text-muted-foreground">未 clone 的远端库</span>
                  <Badge variant="outline">{result.missing?.length ?? 0}</Badge>
                </div>
                {(result.missing ?? []).map((r) => (
                  <div key={r.fullName || r.name} className="flex items-baseline justify-between py-0.5">
                    <span className="font-mono text-xs">{r.fullName || r.name}</span>
                    <span className="font-mono text-[10px] text-muted-foreground">{r.defaultBranch}</span>
                  </div>
                ))}
                {(result.missing?.length ?? 0) === 0 && <span className="text-xs text-muted-foreground">无</span>}
              </div>
              <div>
                <div className="mb-1 flex items-baseline gap-2">
                  <span className="text-xs font-medium text-muted-foreground">本地孤儿（远端已无）</span>
                  <Badge variant="outline">{result.orphan?.length ?? 0}</Badge>
                </div>
                {(result.orphan ?? []).map((l) => (
                  <div key={l.path} className="py-0.5 font-mono text-xs">
                    {l.name}
                  </div>
                ))}
                {(result.orphan?.length ?? 0) === 0 && <span className="text-xs text-muted-foreground">无</span>}
              </div>
              <div>
                <div className="mb-1 flex items-baseline gap-2">
                  <span className="text-xs font-medium text-muted-foreground">已 clone</span>
                  <Badge variant="outline">{result.synced?.length ?? 0}</Badge>
                </div>
                {(result.synced ?? []).map((p) => (
                  <div key={p.local.path} className="flex items-center gap-1.5 py-0.5">
                    <span className="font-mono text-xs">{p.local.name}</span>
                    {p.local.dirty && (
                      <Badge variant="outline" className="text-amber-600">
                        dirty
                      </Badge>
                    )}
                    {p.local.ahead > 0 && <Badge variant="outline">↑{p.local.ahead}</Badge>}
                    {p.local.behind > 0 && <Badge variant="outline">↓{p.local.behind}</Badge>}
                  </div>
                ))}
                {(result.synced?.length ?? 0) === 0 && <span className="text-xs text-muted-foreground">无</span>}
              </div>
            </>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}

export function ForgeNamespacesSection() {
  const namespaces = useForgeNamespaces();
  const forgesQ = useForges();
  const accountsQ = useForgeAccounts();
  const del = useForgeNamespaceDelete();
  const fetchMut = useForgeNamespaceFetch();
  const [editing, setEditing] = useState<NamespaceDraft | null>(null);
  const [deleting, setDeleting] = useState<ForgeNamespace | null>(null);
  const [reconciling, setReconciling] = useState<ForgeNamespace | null>(null);
  const [fetchedHint, setFetchedHint] = useState('');

  const list = namespaces.data?.list ?? [];

  // 拉取成功后直接打开对账抽屉（拉取本身就是对账的前置动作）
  const fetchAndReconcile = (ns: ForgeNamespace) => {
    fetchMut.mutate(
      { forgeHost: ns.forgeHost, path: ns.path, force: true },
      {
        onSuccess: (data) => {
          setFetchedHint(`已拉取 ${ns.path}@${ns.forgeHost}：${data?.count ?? 0} 个仓库`);
          setReconciling(ns);
        },
      },
    );
  };

  return (
    <section>
      <div className="mb-2 flex items-baseline gap-2">
        <h2 className="text-sm font-medium">Forge 命名空间（仓库归属）</h2>
        <span className="text-xs text-muted-foreground">
          按 path 前缀对账远端仓库与本地项目；拉取为出站 API 调用，成功后展示对账结果
        </span>
        <Button
          size="sm"
          variant="outline"
          className="ml-auto"
          onClick={() =>
            setEditing({
              ...EMPTY_NS_DRAFT,
              forgeHost: (forgesQ.data?.list ?? []).filter((f) => f.kind !== 'generic')[0]?.host ?? '',
            })
          }
        >
          新增
        </Button>
      </div>
      {namespaces.error && <ErrorBanner message={`加载失败：${namespaces.error.message}`} />}
      {del.error && <ErrorBanner message={`删除失败：${del.error.message}`} />}
      {fetchMut.error && <ErrorBanner message={`拉取失败：${fetchMut.error.message}`} />}
      {fetchedHint && <span className="text-xs text-muted-foreground">{fetchedHint}</span>}
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>forge</TableHead>
              <TableHead>path</TableHead>
              <TableHead>type</TableHead>
              <TableHead>账号</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {list.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-xs text-muted-foreground">
                  暂无命名空间
                </TableCell>
              </TableRow>
            )}
            {list.map((ns: ForgeNamespace) => (
              <TableRow key={`${ns.forgeHost}/${ns.path}`}>
                <TableCell className="font-mono text-xs">{ns.forgeHost}</TableCell>
                <TableCell className="font-mono text-xs">{ns.path}</TableCell>
                <TableCell>
                  <Badge variant="outline" className="font-mono">
                    {ns.type}
                  </Badge>
                </TableCell>
                <TableCell className="font-mono text-xs">{ns.accountUsername || '—'}</TableCell>
                <TableCell className="text-right">
                  <Button size="sm" variant="ghost" disabled={fetchMut.isPending} onClick={() => fetchAndReconcile(ns)}>
                    拉取对账
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      setEditing({
                        forgeHost: ns.forgeHost,
                        path: ns.path,
                        type: ns.type,
                        accountUsername: ns.accountUsername ?? '',
                      })
                    }
                  >
                    编辑
                  </Button>
                  <Button size="sm" variant="ghost" disabled={del.isPending} onClick={() => setDeleting(ns)}>
                    删除
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <ConfirmDialog
        open={deleting !== null}
        title="删除命名空间"
        message={`确定删除「${deleting?.path}@${deleting?.forgeHost}」吗？仅删除配置，不影响远端与本地仓库。`}
        confirmText="删除"
        danger
        onConfirm={() => {
          if (deleting) del.mutate({ forgeHost: deleting.forgeHost, path: deleting.path });
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />
      {editing && (
        <NamespaceForm
          key={editing.path === '' ? 'new' : `edit:${editing.forgeHost}/${editing.path}`}
          draft={editing}
          forges={forgesQ.data?.list ?? []}
          accountUsernames={(accountsQ.data?.list ?? [])
            .filter((a) => a.forgeHost === editing.forgeHost)
            .map((a) => a.username)}
          onClose={() => setEditing(null)}
        />
      )}
      {reconciling && <ReconcileSheet ns={reconciling} onClose={() => setReconciling(null)} />}
    </section>
  );
}
