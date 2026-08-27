// settings 页「项目·扫描」分区：扫描规则的增删改与拖拽排序。
// 数据源是 /api/project/scan-rules（settings.json scanRules 节），保存即生效。
// 交互模板沿用 Opener 分区（提案 1025 定型）：Sheet 抽屉编辑、删除前确认、grip 拖拽排序。
import { GripVertical } from 'lucide-react';
import { useState } from 'react';

import type { ScanRule } from '@/api/client';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { ErrorBanner } from '@/components/error-banner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useScanRuleDelete, useScanRuleReorder, useScanRuleSave, useScanRules } from '@/queries/scan-rule';

import { STICKY_LEFT, STICKY_RIGHT, useDragOrder } from './drag-order';

interface ScanDraft {
  group: string;
  path: string;
  maxDepth: string;
}

const EMPTY_SCAN_DRAFT: ScanDraft = { group: '', path: '', maxDepth: '3' };

// 编辑表单（右侧抽屉）：关闭即放弃草稿，保存成功后自动关闭
function ScanRuleForm({ draft, onClose }: { draft: ScanDraft; onClose: () => void }) {
  const [form, setForm] = useState<ScanDraft>(draft);
  const save = useScanRuleSave();
  const del = useScanRuleDelete();
  const set = <K extends keyof ScanDraft>(key: K, value: ScanDraft[K]) => setForm((f) => ({ ...f, [key]: value }));
  const isEdit = draft.path !== '';

  const submit = () => {
    const newPath = form.path.trim();
    save.mutate(
      { group: form.group.trim(), path: newPath, maxDepth: Number(form.maxDepth) || 0 },
      {
        onSuccess: () => {
          // path 是规则唯一键：编辑改了 path 等价于删旧存新，补一步删旧
          if (isEdit && newPath !== draft.path) del.mutate({ path: draft.path });
          onClose();
        },
      },
    );
  };

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full gap-0 overflow-y-auto sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{isEdit ? '编辑扫描规则' : '新增扫描规则'}</SheetTitle>
          <SheetDescription>保存即生效（settings.json），项目列表立即重扫</SheetDescription>
        </SheetHeader>
        <div className="flex flex-col gap-4 p-4 text-sm">
          {save.error && <ErrorBanner message={`保存失败：${save.error.message}`} />}

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">group（扫描出的项目组名）</span>
            <Input value={form.group} onChange={(e) => set('group', e.target.value)} placeholder="work" />
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">path（扫描根目录，绝对路径或 ~/ 前缀；规则唯一键）</span>
            <Input
              value={form.path}
              onChange={(e) => set('path', e.target.value)}
              placeholder="~/Code/work"
              className="font-mono"
            />
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">maxDepth（扫描最大深度）</span>
            <Input
              type="number"
              min={1}
              value={form.maxDepth}
              onChange={(e) => set('maxDepth', e.target.value)}
              className="w-24"
            />
          </label>

          <div className="mt-2 flex justify-end gap-2">
            <Button size="sm" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button size="sm" disabled={save.isPending || !form.group.trim() || !form.path.trim()} onClick={submit}>
              保存
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}

export function ScanSection() {
  const rules = useScanRules();
  const del = useScanRuleDelete();
  const reorder = useScanRuleReorder();
  const [editing, setEditing] = useState<ScanDraft | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);

  const list = rules.data?.list ?? [];
  const d = useDragOrder(
    (r: ScanRule) => r.path,
    list,
    (rows) => reorder.mutate({ paths: rows.map((r) => r.path) }),
  );

  return (
    <section>
      <div className="mb-2 flex items-baseline gap-2">
        <h2 className="text-sm font-medium">扫描规则（scanRules）</h2>
        <span className="text-xs text-muted-foreground">
          决定项目列表来源（group / 根目录 / 深度）；顺序即展示序，拖动 ⠿ 排序
        </span>
        <Button size="sm" variant="outline" className="ml-auto" onClick={() => setEditing(EMPTY_SCAN_DRAFT)}>
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
              <TableHead className={STICKY_LEFT}>group</TableHead>
              <TableHead>path</TableHead>
              <TableHead>maxDepth</TableHead>
              <TableHead className={`${STICKY_RIGHT} text-right`}>操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {d.rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="text-xs text-muted-foreground">
                  暂无扫描规则
                </TableCell>
              </TableRow>
            )}
            {d.rows.map((r, i) => (
              <TableRow key={r.path} {...d.rowProps(r, i)}>
                <TableCell className={`${STICKY_LEFT} font-medium`}>
                  <span className="flex items-center gap-1">
                    <GripVertical {...d.gripProps(r)} />
                    {r.group}
                  </span>
                </TableCell>
                <TableCell className="font-mono text-xs">{r.path}</TableCell>
                <TableCell className="font-mono text-xs">{r.maxDepth}</TableCell>
                <TableCell className={`${STICKY_RIGHT} text-right`}>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setEditing({ group: r.group, path: r.path, maxDepth: String(r.maxDepth) })}
                  >
                    编辑
                  </Button>
                  <Button size="sm" variant="ghost" disabled={del.isPending} onClick={() => setDeleting(r.path)}>
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
        title="删除扫描规则"
        message={`确定删除「${deleting}」吗？该项目组将从列表中移除，操作立即生效。`}
        confirmText="删除"
        danger
        onConfirm={() => {
          if (deleting) del.mutate({ path: deleting });
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />
      {editing && (
        <ScanRuleForm
          key={editing === EMPTY_SCAN_DRAFT ? 'new' : `edit:${editing.path}`}
          draft={editing}
          onClose={() => setEditing(null)}
        />
      )}
    </section>
  );
}
