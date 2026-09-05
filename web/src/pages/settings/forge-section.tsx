// settings 页「Forge」分区：git 托管平台实例（host 级）配置的增删改（提案 1040）。
// 数据源 /api/forge/list（settings.json forges 节），保存即生效。
// 交互模板沿用扫描规则分区：Sheet 抽屉编辑、删除前确认；forge 数量少、无顺序语义，不做拖拽排序。
import { useState } from 'react';

import type { Forge } from '@/api/client';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { ErrorBanner } from '@/components/error-banner';
import { IconField } from '@/components/icon-field';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import type { IconDecl } from '@/lib/icon';
import { renderIcon } from '@/lib/icon';
import { cn } from '@/lib/utils';
import { useForgeDelete, useForgeSave, useForges } from '@/queries/forge';

interface ForgeDraft {
  host: string;
  kind: string;
  icon?: IconDecl;
}

const EMPTY_FORGE_DRAFT: ForgeDraft = { host: '', kind: 'github' };

// kind 固定四值：决定 API 方言与 1041 account 拉取能力（generic = 无 API 仅展示）
const KINDS: { value: string; label: string; hint: string }[] = [
  { value: 'github', label: 'github', hint: 'GitHub' },
  { value: 'gitea', label: 'gitea', hint: 'Gitea（自建常见）' },
  { value: 'gitee', label: 'gitee', hint: 'Gitee 码云' },
  { value: 'generic', label: 'generic', hint: '通用，无 API 仅展示' },
];

// 编辑表单（右侧抽屉）：关闭即放弃草稿，保存成功后自动关闭
function ForgeForm({ draft, onClose }: { draft: ForgeDraft; onClose: () => void }) {
  const [form, setForm] = useState<ForgeDraft>(draft);
  const save = useForgeSave();
  const del = useForgeDelete();
  const set = <K extends keyof ForgeDraft>(key: K, value: ForgeDraft[K]) => setForm((f) => ({ ...f, [key]: value }));
  const isEdit = draft.host !== '';

  const submit = () => {
    const newHost = form.host.trim();
    save.mutate(
      {
        host: newHost,
        kind: form.kind,
        icon: form.icon && form.icon.value ? { type: form.icon.type, value: form.icon.value } : undefined,
      },
      {
        onSuccess: () => {
          // host 是唯一键：编辑改了 host 等价于删旧存新，补一步删旧
          if (isEdit && newHost !== draft.host) del.mutate({ host: draft.host });
          onClose();
        },
      },
    );
  };

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full gap-0 overflow-y-auto sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{isEdit ? '编辑 forge' : '新增 forge'}</SheetTitle>
          <SheetDescription>保存即生效（settings.json），项目列表按 host 匹配展示图标</SheetDescription>
        </SheetHeader>
        <div className="flex flex-col gap-4 p-4 text-sm">
          {save.error && <ErrorBanner message={`保存失败：${save.error.message}`} />}

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">host（域名，可带端口；规则唯一键）</span>
            <Input
              value={form.host}
              onChange={(e) => set('host', e.target.value)}
              placeholder="github.com"
              className="font-mono"
            />
          </label>

          <div className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">kind（API 方言，决定账号拉取能力）</span>
            <div className="flex flex-wrap gap-1.5">
              {KINDS.map((k) => (
                <button
                  key={k.value}
                  type="button"
                  title={k.hint}
                  onClick={() => set('kind', k.value)}
                  className={cn(
                    'rounded-full border px-2.5 py-0.5 font-mono text-xs transition-colors',
                    form.kind === k.value
                      ? 'border-primary bg-primary text-primary-foreground'
                      : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                  )}
                >
                  {k.label}
                </button>
              ))}
            </div>
          </div>

          <div className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">icon（可选，项目列表按 host 匹配展示）</span>
            <IconField allowEmpty value={form.icon} onChange={(v) => set('icon', v)} />
          </div>

          <div className="mt-2 flex justify-end gap-2">
            <Button size="sm" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button size="sm" disabled={save.isPending || !form.host.trim()} onClick={submit}>
              保存
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}

export function ForgeSection() {
  const forges = useForges();
  const del = useForgeDelete();
  const [editing, setEditing] = useState<ForgeDraft | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);

  const list = forges.data?.list ?? [];

  return (
    <section>
      <div className="mb-2 flex items-baseline gap-2">
        <h2 className="text-sm font-medium">Forge（git 托管平台）</h2>
        <span className="text-xs text-muted-foreground">
          host 级平台实例（github.com / 自建 gitea 等）；项目按 repo host 匹配展示图标，account 拉取（1041）依赖 kind
        </span>
        <Button size="sm" variant="outline" className="ml-auto" onClick={() => setEditing(EMPTY_FORGE_DRAFT)}>
          新增
        </Button>
      </div>
      {forges.error && <ErrorBanner message={`加载失败：${forges.error.message}`} />}
      {del.error && <ErrorBanner message={`删除失败：${del.error.message}`} />}
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>forge</TableHead>
              <TableHead>kind</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {list.length === 0 && (
              <TableRow>
                <TableCell colSpan={3} className="text-xs text-muted-foreground">
                  暂无 forge 配置
                </TableCell>
              </TableRow>
            )}
            {list.map((f: Forge) => (
              <TableRow key={f.host}>
                <TableCell className="font-medium">
                  <span className="flex items-center gap-1.5">
                    {renderIcon(f.icon ? { type: f.icon.type, value: f.icon.value } : undefined, null)}
                    <span className="font-mono text-xs">{f.host}</span>
                  </span>
                </TableCell>
                <TableCell>
                  <Badge variant="outline" className="font-mono">
                    {f.kind}
                  </Badge>
                </TableCell>
                <TableCell className="text-right">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      setEditing({
                        host: f.host,
                        kind: f.kind,
                        icon: f.icon ? { type: f.icon.type, value: f.icon.value } : undefined,
                      })
                    }
                  >
                    编辑
                  </Button>
                  <Button size="sm" variant="ghost" disabled={del.isPending} onClick={() => setDeleting(f.host)}>
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
        title="删除 forge"
        message={`确定删除「${deleting}」吗？项目列表将不再展示该平台图标，操作立即生效。`}
        confirmText="删除"
        danger
        onConfirm={() => {
          if (deleting) del.mutate({ host: deleting });
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />
      {editing && (
        <ForgeForm
          key={editing === EMPTY_FORGE_DRAFT ? 'new' : `edit:${editing.host}`}
          draft={editing}
          onClose={() => setEditing(null)}
        />
      )}
    </section>
  );
}
