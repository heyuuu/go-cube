// settings 页「项目·扫描」分区：扫描规则 + clone 规则两组表格的增删改与拖拽排序。
// 数据源是 /api/project/scan-rules 与 clone-rules（settings.json），保存即生效。
// 交互模板沿用 Opener 分区（提案 1025 定型）：Sheet 抽屉编辑、删除前确认、grip 拖拽排序。
import { GripVertical } from 'lucide-react';
import { useState } from 'react';

import type { CloneRule, ScanRule } from '@/api/client';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { ErrorBanner } from '@/components/error-banner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';
import {
  useCloneRuleDelete,
  useCloneRuleReorder,
  useCloneRuleSave,
  useCloneRules,
  useScanRuleDelete,
  useScanRuleReorder,
  useScanRuleSave,
  useScanRules,
} from '@/queries/scan-rule';

// 冻结列样式（同 opener 分区）：首列贴左、操作贴右，实心 bg 遮住下层滑过的单元格
const STICKY_LEFT =
  'sticky left-0 z-10 bg-background group-hover/row:bg-muted/50 shadow-[inset_-1px_0_0_var(--border)]';
const STICKY_RIGHT =
  'sticky right-0 z-10 bg-background group-hover/row:bg-muted/50 shadow-[inset_1px_0_0_var(--border)]';

// 拖拽排序（逻辑同 opener 分区）：armed 记录「按住 grip 的行」——只有手柄能发起拖拽
// （整行 draggable 会干扰按钮点击与文本选择）；order 是乐观顺序，与服务端列表长度
// 不一致（增删后）即失效回落。onReorder 直接回传排好序的行，调用方自行映射提交。
function useDragOrder<T>(keyOf: (item: T) => string, list: T[], onReorder: (rows: T[]) => void) {
  const [order, setOrder] = useState<string[] | null>(null);
  const [armed, setArmed] = useState<string | null>(null);
  const [dragIdx, setDragIdx] = useState<number | null>(null);
  const [overIdx, setOverIdx] = useState<number | null>(null);

  // 个位数列表，重排计算不值得 useMemo（直接算还免去 list 引用不稳的依赖告警）
  const rows = (() => {
    if (order === null || order.length !== list.length) return list;
    const byKey = new Map(list.map((item) => [keyOf(item), item] as const));
    const sorted: T[] = [];
    for (const k of order) {
      const hit = byKey.get(k);
      if (!hit) return list;
      sorted.push(hit);
    }
    return sorted;
  })();

  const dropTo = (target: number) => {
    if (dragIdx === null || dragIdx === target) return;
    const next = [...rows];
    // 插入位语义：从上往下拖插到 target 之后、从下往上拖插到 target 之前（与高亮线一致）
    const insertAt = dragIdx < target ? target + 1 : target;
    const [moved] = next.splice(dragIdx, 1);
    next.splice(insertAt, 0, moved);
    setOrder(next.map(keyOf));
    onReorder(next);
  };

  const reset = () => {
    setArmed(null);
    setDragIdx(null);
    setOverIdx(null);
  };

  const rowProps = (item: T, i: number) => ({
    className: cn(
      'group/row',
      i === dragIdx && 'opacity-40',
      // 插入位高亮：inset 阴影画线，避免 border 变宽引起行高跳动
      dragIdx !== null &&
        i === overIdx &&
        i !== dragIdx &&
        (dragIdx < i ? 'shadow-[inset_0_-2px_0_var(--primary)]' : 'shadow-[inset_0_2px_0_var(--primary)]'),
    ),
    draggable: armed === keyOf(item),
    onDragStart: (e: React.DragEvent) => {
      setDragIdx(i);
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', keyOf(item));
    },
    onDragOver: (e: React.DragEvent) => {
      e.preventDefault();
      setOverIdx(i);
    },
    onDrop: (e: React.DragEvent) => {
      e.preventDefault();
      dropTo(i);
    },
    onDragEnd: reset,
  });

  // grip 手柄属性：按下才 armed（行变为 draggable），松开解除
  const gripProps = (item: T) => ({
    className: 'size-3.5 shrink-0 cursor-grab text-muted-foreground/40',
    onPointerDown: () => setArmed(keyOf(item)),
    onPointerUp: () => setArmed(null),
    'aria-label': '拖动排序',
  });

  return { rows, rowProps, gripProps };
}

export function ScanSection() {
  return (
    <div className="flex flex-col gap-6">
      <p className="text-xs text-muted-foreground">
        扫描规则决定项目列表的来源（group / 根目录 / 深度）；clone 规则决定 cube clone
        的落地路径。保存即生效（settings.json），无需重启
      </p>
      <ScanRulesTable />
      <CloneRulesTable />
    </div>
  );
}

// --- 扫描规则 ---

interface ScanDraft {
  group: string;
  path: string;
  maxDepth: string;
}

const EMPTY_SCAN_DRAFT: ScanDraft = { group: '', path: '', maxDepth: '3' };

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

function ScanRulesTable() {
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
        <span className="text-xs text-muted-foreground">顺序即项目列表展示序；拖动 ⠿ 排序</span>
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

// --- clone 规则 ---

interface CloneDraft {
  repoHost: string;
  repoPrefix: string;
  localPath: string;
}

const EMPTY_CLONE_DRAFT: CloneDraft = { repoHost: '', repoPrefix: '', localPath: '' };

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

function CloneRulesTable() {
  const rules = useCloneRules();
  const del = useCloneRuleDelete();
  const reorder = useCloneRuleReorder();
  const [editing, setEditing] = useState<CloneDraft | null>(null);
  const [deleting, setDeleting] = useState<CloneRule | null>(null);

  const list = rules.data?.list ?? [];
  const d = useDragOrder(
    (r: CloneRule) => `${r.repoHost}|${r.repoPrefix ?? ''}`,
    list,
    (rows) => reorder.mutate({ rules: rows.map((r) => ({ repoHost: r.repoHost, repoPrefix: r.repoPrefix ?? '' })) }),
  );
  const keyOf = (r: CloneRule) => `${r.repoHost}${r.repoPrefix ?? ''}`;

  return (
    <section>
      <div className="mb-2 flex items-baseline gap-2">
        <h2 className="text-sm font-medium">clone 路由（cloneRules）</h2>
        <span className="text-xs text-muted-foreground">按 repoHost / repoPrefix 匹配落地路径；前缀最长优先</span>
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
              <TableRow key={keyOf(r)} {...d.rowProps(r, i)}>
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
        message={`确定删除「${deleting ? keyOf(deleting) : ''}」吗？操作立即生效。`}
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
          key={editing === EMPTY_CLONE_DRAFT ? 'new' : `edit:${keyOf(editing)}`}
          draft={editing}
          onClose={() => setEditing(null)}
        />
      )}
    </section>
  );
}
