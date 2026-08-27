// settings 页「Clone 路由」分区：clone 规则的增删改与拖拽排序。
// 数据源是 /api/project/clone-rules（settings.json cloneRules 节），保存即生效。
// 交互模板沿用 Opener 分区（提案 1025 定型）：Sheet 抽屉编辑、删除前确认、grip 拖拽排序。
import { GripVertical } from 'lucide-react';
import { useState } from 'react';

import type { CloneRule } from '@/api/client';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { ErrorBanner } from '@/components/error-banner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useCloneRuleDelete, useCloneRuleReorder, useCloneRuleSave, useCloneRules } from '@/queries/scan-rule';

import { STICKY_LEFT, STICKY_RIGHT, useDragOrder } from './drag-order';

interface CloneDraft {
  repoHost: string;
  repoPrefix: string;
  localPath: string;
}

const EMPTY_CLONE_DRAFT: CloneDraft = { repoHost: '', repoPrefix: '', localPath: '' };

// host|prefix 作拖拽键（host 不含 |，可无损拼接）；对用户展示的键是 host+prefix 串
const dragKeyOf = (r: CloneRule) => `${r.repoHost}|${r.repoPrefix ?? ''}`;
const displayKeyOf = (r: CloneRule) => `${r.repoHost}${r.repoPrefix ?? ''}`;

// 编辑表单（右侧抽屉）：关闭即放弃草稿，保存成功后自动关闭
function CloneRuleForm({ draft, onClose }: { draft: CloneDraft; onClose: () => void }) {
  const [form, setForm] = useState<CloneDraft>(draft);
  const save = useCloneRuleSave();
  const set = <K extends keyof CloneDraft>(key: K, value: CloneDraft[K]) => setForm((f) => ({ ...f, [key]: value }));

  const submit = () => {
    save.mutate(
      {
        repoHost: form.repoHost.trim(),
        repoPrefix: form.repoPrefix.trim() || undefined,
        localPath: form.localPath.trim(),
      },
      { onSuccess: onClose },
    );
  };

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full gap-0 overflow-y-auto sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{draft.repoHost ? '编辑 clone 规则' : '新增 clone 规则'}</SheetTitle>
          <SheetDescription>保存即生效（settings.json），无需重启</SheetDescription>
        </SheetHeader>
        <div className="flex flex-col gap-4 p-4 text-sm">
          {save.error && <ErrorBanner message={`保存失败：${save.error.message}`} />}

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">repoHost（源域名，无协议；与 repoPrefix 组成唯一键）</span>
            <Input value={form.repoHost} onChange={(e) => set('repoHost', e.target.value)} placeholder="github.com" />
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">repoPrefix（uri 前缀，须以 / 开头或留空）</span>
            <Input
              value={form.repoPrefix}
              onChange={(e) => set('repoPrefix', e.target.value)}
              placeholder="/heyuuu"
              className="font-mono"
            />
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">localPath（对应本地目录，绝对路径或 ~/ 前缀）</span>
            <Input
              value={form.localPath}
              onChange={(e) => set('localPath', e.target.value)}
              placeholder="~/Code/gh"
              className="font-mono"
            />
          </label>

          <div className="mt-2 flex justify-end gap-2">
            <Button size="sm" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button
              size="sm"
              disabled={save.isPending || !form.repoHost.trim() || !form.localPath.trim()}
              onClick={submit}
            >
              保存
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}

export function CloneSection() {
  const rules = useCloneRules();
  const del = useCloneRuleDelete();
  const reorder = useCloneRuleReorder();
  const [editing, setEditing] = useState<CloneDraft | null>(null);
  const [deleting, setDeleting] = useState<CloneRule | null>(null);

  const list = rules.data?.list ?? [];
  const d = useDragOrder(dragKeyOf, list, (rows) =>
    reorder.mutate({ rules: rows.map((r) => ({ repoHost: r.repoHost, repoPrefix: r.repoPrefix ?? '' })) }),
  );

  return (
    <section>
      <div className="mb-2 flex items-baseline gap-2">
        <h2 className="text-sm font-medium">clone 路由（cloneRules）</h2>
        <span className="text-xs text-muted-foreground">
          按 repoHost / repoPrefix 匹配 cube clone 的落地路径；前缀最长优先
        </span>
        <Button size="sm" variant="outline" className="ml-auto" onClick={() => setEditing(EMPTY_CLONE_DRAFT)}>
          新增
        </Button>
      </div>
      {rules.error && <ErrorBanner message={`加载失败：${rules.error.message}`} />}
      {del.error && <ErrorBanner message={`删除失败：${del.error.message}`} />}
      {reorder.error && <ErrorBanner message={`排序失败：${reorder.error.message}`} />}
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className={STICKY_LEFT}>repoHost</TableHead>
              <TableHead>repoPrefix</TableHead>
              <TableHead>localPath</TableHead>
              <TableHead className={`${STICKY_RIGHT} text-right`}>操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {d.rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="text-xs text-muted-foreground">
                  暂无 clone 规则
                </TableCell>
              </TableRow>
            )}
            {d.rows.map((r, i) => (
              <TableRow key={dragKeyOf(r)} {...d.rowProps(r, i)}>
                <TableCell className={`${STICKY_LEFT} font-medium`}>
                  <span className="flex items-center gap-1">
                    <GripVertical {...d.gripProps(r)} />
                    {r.repoHost}
                  </span>
                </TableCell>
                <TableCell className="font-mono text-xs">{r.repoPrefix || '-'}</TableCell>
                <TableCell className="font-mono text-xs">{r.localPath}</TableCell>
                <TableCell className={`${STICKY_RIGHT} text-right`}>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      setEditing({ repoHost: r.repoHost, repoPrefix: r.repoPrefix ?? '', localPath: r.localPath })
                    }
                  >
                    编辑
                  </Button>
                  <Button size="sm" variant="ghost" disabled={del.isPending} onClick={() => setDeleting(r)}>
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
        title="删除 clone 规则"
        message={`确定删除「${deleting ? displayKeyOf(deleting) : ''}」吗？操作立即生效。`}
        confirmText="删除"
        danger
        onConfirm={() => {
          if (deleting) del.mutate({ repoHost: deleting.repoHost, repoPrefix: deleting.repoPrefix ?? '' });
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />
      {editing && (
        <CloneRuleForm
          key={editing === EMPTY_CLONE_DRAFT ? 'new' : `edit:${displayKeyOf(editing)}`}
          draft={editing}
          onClose={() => setEditing(null)}
        />
      )}
    </section>
  );
}
