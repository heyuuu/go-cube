// settings 页 Forge 分区「账号」子表：forge account（纯 API 凭证）的增删改与拖拽排序（提案 1041/1044）。
// token 只以掩码形态展示与回传（提交掩码值 = 未修改，沿用旧值）；generic forge 无 API 不可挂账号。
// 拖动 ⠿ 排序，顺序即 forge 页账号摘要条的展示序。
import { GripVertical } from 'lucide-react';
import { useState } from 'react';

import type { Forge, ForgeAccount } from '@/api/client';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { ErrorBanner } from '@/components/error-banner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';
import {
  useForgeAccountDelete,
  useForgeAccountReorder,
  useForgeAccountSave,
  useForgeAccounts,
  useForges,
} from '@/queries/forge';

import { STICKY_LEFT, STICKY_RIGHT, useDragOrder } from './drag-order';

interface AccountDraft {
  forgeHost: string;
  username: string;
  token: string;
}

const EMPTY_ACCOUNT_DRAFT: AccountDraft = { forgeHost: '', username: '', token: '' };

function AccountForm({ draft, forges, onClose }: { draft: AccountDraft; forges: Forge[]; onClose: () => void }) {
  const [form, setForm] = useState<AccountDraft>(draft);
  const save = useForgeAccountSave();
  const set = <K extends keyof AccountDraft>(key: K, value: AccountDraft[K]) =>
    setForm((f) => ({ ...f, [key]: value }));
  const isEdit = draft.username !== '';
  // 只能挂到有 API 的 forge（generic 无 API，凭证无意义）
  const apiForges = forges.filter((f) => f.kind !== 'generic');

  const submit = () => {
    save.mutate(
      { forgeHost: form.forgeHost, username: form.username.trim(), token: form.token },
      { onSuccess: onClose },
    );
  };

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full gap-0 overflow-y-auto sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{isEdit ? '编辑账号' : '新增账号'}</SheetTitle>
          <SheetDescription>token 仅用于调平台 API 拉取仓库列表（含私有库），git 操作仍走本机凭证</SheetDescription>
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
                  className={cn(
                    'rounded-full border px-2.5 py-0.5 font-mono text-xs transition-colors',
                    form.forgeHost === f.host
                      ? 'border-primary bg-primary text-primary-foreground'
                      : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                  )}
                >
                  {f.host}
                </button>
              ))}
            </div>
          </div>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">username（平台用户名，唯一键之一）</span>
            <Input
              value={form.username}
              onChange={(e) => set('username', e.target.value)}
              placeholder="heyuuu"
              className="font-mono"
              disabled={isEdit}
            />
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">token（留空 = 仅公开数据；编辑时保持掩码 = 未修改）</span>
            <Input
              type="password"
              value={form.token}
              onChange={(e) => set('token', e.target.value)}
              placeholder={isEdit ? '••••••' : 'ghp_xxx'}
              className="font-mono"
            />
          </label>

          <div className="mt-2 flex justify-end gap-2">
            <Button size="sm" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button size="sm" disabled={save.isPending || !form.forgeHost || !form.username.trim()} onClick={submit}>
              保存
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}

export function ForgeAccountsSection() {
  const accounts = useForgeAccounts();
  const forgesQ = useForges();
  const del = useForgeAccountDelete();
  const reorder = useForgeAccountReorder();
  const [editing, setEditing] = useState<AccountDraft | null>(null);
  const [deleting, setDeleting] = useState<ForgeAccount | null>(null);

  const list = accounts.data?.list ?? [];
  const d = useDragOrder(
    (a: ForgeAccount) => `${a.forgeHost}/${a.username}`,
    list,
    (rows) => reorder.mutate({ keys: rows.map((r) => `${r.forgeHost}/${r.username}`) }),
  );

  return (
    <section>
      <div className="mb-2 flex items-baseline gap-2">
        <h2 className="text-sm font-medium">Forge 账号（API 凭证）</h2>
        <span className="text-xs text-muted-foreground">
          平台账号的 API token，供 namespace 拉取私有库；一个 forge 可挂多个账号
        </span>
        <Button
          size="sm"
          variant="outline"
          className="ml-auto"
          onClick={() =>
            setEditing({
              ...EMPTY_ACCOUNT_DRAFT,
              forgeHost: (forgesQ.data?.list ?? []).filter((f) => f.kind !== 'generic')[0]?.host ?? '',
            })
          }
        >
          新增
        </Button>
      </div>
      {accounts.error && <ErrorBanner message={`加载失败：${accounts.error.message}`} />}
      {del.error && <ErrorBanner message={`删除失败：${del.error.message}`} />}
      {reorder.error && <ErrorBanner message={`排序失败：${reorder.error.message}`} />}
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>forge</TableHead>
              <TableHead>username</TableHead>
              <TableHead>token</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {list.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="text-xs text-muted-foreground">
                  暂无账号（不配 token 也能拉取公开 namespace）
                </TableCell>
              </TableRow>
            )}
            {d.rows.map((a: ForgeAccount, i) => (
              <TableRow key={`${a.forgeHost}/${a.username}`} {...d.rowProps(a, i)}>
                <TableCell className={STICKY_LEFT}>
                  <span className="flex items-center gap-1.5">
                    <GripVertical {...d.gripProps(a)} />
                    <span className="font-mono text-xs">{a.forgeHost}</span>
                  </span>
                </TableCell>
                <TableCell className="font-mono text-xs">{a.username}</TableCell>
                <TableCell>
                  <Badge variant="outline" className="font-mono">
                    {a.token || '未设置'}
                  </Badge>
                </TableCell>
                <TableCell className={`${STICKY_RIGHT} text-right`}>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setEditing({ forgeHost: a.forgeHost, username: a.username, token: a.token })}
                  >
                    编辑
                  </Button>
                  <Button size="sm" variant="ghost" disabled={del.isPending} onClick={() => setDeleting(a)}>
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
        title="删除账号"
        message={`确定删除「${deleting?.username}@${deleting?.forgeHost}」吗？挂载它的 namespace 将退化为仅拉取公开数据。`}
        confirmText="删除"
        danger
        onConfirm={() => {
          if (deleting) del.mutate({ forgeHost: deleting.forgeHost, username: deleting.username });
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />
      {editing && (
        <AccountForm
          key={editing.username === '' ? 'new' : `edit:${editing.forgeHost}/${editing.username}`}
          draft={editing}
          forges={forgesQ.data?.list ?? []}
          onClose={() => setEditing(null)}
        />
      )}
    </section>
  );
}
