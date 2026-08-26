// settings 页 Opener 分区：列表 + 抽屉（Sheet）编辑表单，删除前确认。
// 数据源是 /api/opener/list（settings.json），保存即生效。
import { GripVertical } from 'lucide-react';
import { useState } from 'react';

import type { Opener } from '@/api/client';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { ErrorBanner } from '@/components/error-banner';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { renderOpenerIcon } from '@/lib/opener-icon';
import { cn } from '@/lib/utils';
import { useIconExtract, useOpenerDelete, useOpenerReorder, useOpenerSave } from '@/queries/opener';
import { useOpenerList } from '@/queries/project';

import { LucideIconPicker } from './lucide-icon-picker';

const ALL_ROLES = ['open-dir', 'open-file', 'diff-dir', 'diff-file'] as const;

// 冻结列：name 贴左、操作贴右，水平滚动时始终可见。实心 bg 遮住下层滑过的单元格，
// 分隔线用 inset 阴影而非 border（border-collapse 下 border 不随 sticky 单元格移动）；
// 行 hover 靠 group 保持整行联动（sticky 单元格自身的 bg 会盖掉 tr 的 hover bg）
const STICKY_LEFT =
  'sticky left-0 z-10 bg-background group-hover/row:bg-muted/50 shadow-[inset_-1px_0_0_var(--border)]';
const STICKY_RIGHT =
  'sticky right-0 z-10 bg-background group-hover/row:bg-muted/50 shadow-[inset_1px_0_0_var(--border)]';

// 表单草稿：cmd 以空格分隔编辑（v1 约定：cmd 参数不含空格）
interface Draft {
  name: string;
  title: string;
  cmd: string;
  roles: string[];
  iconType: 'lucide' | 'image';
  iconValue: string;
}

// 与后端 defaultIcon 一致（opener/icon.go）：icon 必有值，新增默认 lucide:app-window-mac
const DEFAULT_LUCIDE = 'app-window-mac';

const EMPTY_DRAFT: Draft = {
  name: '',
  title: '',
  cmd: '',
  roles: ['open-dir'],
  iconType: 'lucide',
  iconValue: DEFAULT_LUCIDE,
};

function fromOpener(op: Opener): Draft {
  return {
    name: op.name,
    title: op.title === `用 ${op.name} 打开` ? '' : op.title,
    // summary 是命令模板的空格拼接展示，直接还原成编辑文本
    cmd: op.summary,
    roles: op.roles ?? [],
    iconType: op.icon?.type === 'image' ? 'image' : 'lucide',
    iconValue: op.icon?.type === 'image' ? (op.icon.value ?? '') : op.icon?.value || DEFAULT_LUCIDE,
  };
}

// 编辑表单（右侧抽屉）：关闭即放弃草稿，保存成功后自动关闭
function OpenerForm({ draft, onClose }: { draft: Draft; onClose: () => void }) {
  const [form, setForm] = useState<Draft>(draft);
  const save = useOpenerSave();
  const extract = useIconExtract();
  const set = <K extends keyof Draft>(key: K, value: Draft[K]) => setForm((f) => ({ ...f, [key]: value }));
  const [appPath, setAppPath] = useState('');

  const submit = () => {
    save.mutate(
      {
        name: form.name.trim(),
        title: form.title.trim() || undefined,
        cmd: form.cmd.trim().split(/\s+/).filter(Boolean),
        roles: form.roles,
        icon: form.iconType && form.iconValue ? { type: form.iconType, value: form.iconValue } : undefined,
      },
      { onSuccess: onClose },
    );
  };

  const onUpload = (file: File) => {
    const reader = new FileReader();
    reader.onload = () => {
      // dataURL 去掉前缀，存纯 base64
      set('iconValue', String(reader.result).replace(/^data:[^,]*,/, ''));
    };
    reader.readAsDataURL(file);
  };

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full gap-0 overflow-y-auto sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{draft.name ? '编辑 opener' : '新增 opener'}</SheetTitle>
          <SheetDescription>保存即生效（settings.json），无需重启</SheetDescription>
        </SheetHeader>
        <div className="flex flex-col gap-4 p-4 text-sm">
          {save.error && <ErrorBanner message={`保存失败：${save.error.message}`} />}
          {extract.error && <ErrorBanner message={`提取失败：${extract.error.message}`} />}

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">name（唯一标识）</span>
            <Input value={form.name} onChange={(e) => set('name', e.target.value)} placeholder="code" />
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">title（展示文案，缺省生成「用 name 打开」）</span>
            <Input value={form.title} onChange={(e) => set('title', e.target.value)} placeholder="打开所在目录" />
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">
              cmd（空格分隔，$0/$1 占位路径槽位；打开工作台可用 `cube web workbench $0`）
            </span>
            <Input
              value={form.cmd}
              onChange={(e) => set('cmd', e.target.value)}
              placeholder="code $0"
              className="font-mono"
            />
          </label>

          <div className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">roles（用途；slotCount 需一致）</span>
            <div className="flex flex-wrap gap-x-4 gap-y-1">
              {ALL_ROLES.map((r) => (
                <label key={r} className="flex items-center gap-1.5 text-xs">
                  <Checkbox
                    checked={form.roles.includes(r)}
                    onCheckedChange={(v) => set('roles', v ? [...form.roles, r] : form.roles.filter((x) => x !== r))}
                  />
                  {r}
                </label>
              ))}
            </div>
          </div>

          <div className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">icon</span>
            <div className="flex gap-2">
              {(['lucide', 'image'] as const).map((t) => (
                <Button
                  key={t}
                  size="sm"
                  variant={form.iconType === t ? 'default' : 'outline'}
                  onClick={() => {
                    set('iconType', t);
                    // 切换类型换图片来源；lucide 回落默认图，避免「未选择」态
                    set('iconValue', t === 'lucide' ? DEFAULT_LUCIDE : '');
                  }}
                >
                  {t}
                </Button>
              ))}
            </div>
            {form.iconType === 'lucide' && (
              <LucideIconPicker value={form.iconValue} onChange={(v) => set('iconValue', v)} />
            )}
            {form.iconType === 'image' && (
              <div className="flex flex-col gap-2">
                {form.iconValue && (
                  <img src={`data:image/png;base64,${form.iconValue}`} alt="icon 预览" className="size-8 rounded" />
                )}
                <div className="flex items-center gap-2">
                  <Input
                    value={appPath}
                    onChange={(e) => setAppPath(e.target.value)}
                    placeholder="/Applications/Xxx.app"
                  />
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!appPath || extract.isPending}
                    onClick={() => extract.mutate(appPath, { onSuccess: (v) => set('iconValue', v) })}
                  >
                    提取
                  </Button>
                </div>
                <input
                  type="file"
                  accept="image/png"
                  onChange={(e) => e.target.files?.[0] && onUpload(e.target.files[0])}
                  className="text-xs"
                />
              </div>
            )}
          </div>

          <div className="mt-2 flex justify-end gap-2">
            <Button size="sm" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button size="sm" disabled={save.isPending || !form.name.trim()} onClick={submit}>
              保存
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}

export function OpenerSection() {
  const openers = useOpenerList();
  const del = useOpenerDelete();
  const reorder = useOpenerReorder();
  const [editing, setEditing] = useState<Draft | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);

  // 拖拽排序：armed 记录「按住 grip 的行」——只有 grip 手柄能发起拖拽（整行 draggable 会
  // 干扰按钮点击与文本选择）；order 是乐观顺序，与服务端名单集合不一致（增删后）即失效回落
  const list = openers.data?.list ?? [];
  const [order, setOrder] = useState<string[] | null>(null);
  const [armed, setArmed] = useState<string | null>(null);
  const [dragIdx, setDragIdx] = useState<number | null>(null);
  const [overIdx, setOverIdx] = useState<number | null>(null);

  // 个位数列表，重排计算不值得 useMemo（直接算还免去 list 引用不稳的依赖告警）
  const rows = (() => {
    if (order === null || order.length !== list.length) return list;
    const byName = new Map(list.map((op) => [op.name, op] as const));
    const sorted: typeof list = [];
    for (const n of order) {
      const hit = byName.get(n);
      if (!hit) return list;
      sorted.push(hit);
    }
    return sorted;
  })();

  const dropTo = (target: number) => {
    if (dragIdx === null || dragIdx === target) return;
    const names = rows.map((r) => r.name);
    // 插入位语义：从上往下拖插到 target 之后、从下往上拖插到 target 之前（与高亮线一致）
    const insertAt = dragIdx < target ? target + 1 : target;
    const [moved] = names.splice(dragIdx, 1);
    names.splice(insertAt, 0, moved);
    setOrder(names);
    reorder.mutate({ names });
  };

  return (
    <section>
      <div className="mb-2 flex items-baseline gap-2">
        <h2 className="text-sm font-medium">打开工具（openers）</h2>
        <span className="text-xs text-muted-foreground">settings.json，保存即生效；拖动 ⠿ 排序</span>
        <Button size="sm" variant="outline" className="ml-auto" onClick={() => setEditing(EMPTY_DRAFT)}>
          新增
        </Button>
      </div>
      {openers.error && <ErrorBanner message={`加载失败：${openers.error.message}`} />}
      {del.error && <ErrorBanner message={`删除失败：${del.error.message}`} />}
      {reorder.error && <ErrorBanner message={`排序失败：${reorder.error.message}`} />}
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className={STICKY_LEFT}>name</TableHead>
              <TableHead>icon</TableHead>
              <TableHead>title</TableHead>
              <TableHead>cmd</TableHead>
              <TableHead>roles</TableHead>
              <TableHead className={`${STICKY_RIGHT} text-right`}>操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} className="text-xs text-muted-foreground">
                  暂无 opener
                </TableCell>
              </TableRow>
            )}
            {rows.map((op, i) => (
              <TableRow
                key={op.name}
                className={cn(
                  'group/row',
                  i === dragIdx && 'opacity-40',
                  // 插入位高亮：inset 阴影画线，避免 border 变宽引起行高跳动
                  dragIdx !== null &&
                    i === overIdx &&
                    i !== dragIdx &&
                    (dragIdx < i ? 'shadow-[inset_0_-2px_0_var(--primary)]' : 'shadow-[inset_0_2px_0_var(--primary)]'),
                )}
                draggable={armed === op.name}
                onDragStart={(e) => {
                  setDragIdx(i);
                  e.dataTransfer.effectAllowed = 'move';
                  e.dataTransfer.setData('text/plain', op.name);
                }}
                onDragOver={(e) => {
                  e.preventDefault();
                  setOverIdx(i);
                }}
                onDrop={(e) => {
                  e.preventDefault();
                  dropTo(i);
                }}
                onDragEnd={() => {
                  setArmed(null);
                  setDragIdx(null);
                  setOverIdx(null);
                }}
              >
                <TableCell className={`${STICKY_LEFT} font-medium`}>
                  <span className="flex items-center gap-1">
                    <GripVertical
                      className="size-3.5 shrink-0 cursor-grab text-muted-foreground/40"
                      onPointerDown={() => setArmed(op.name)}
                      onPointerUp={() => setArmed(null)}
                      aria-label="拖动排序"
                    />
                    {op.name}
                  </span>
                </TableCell>
                <TableCell>{renderOpenerIcon(op, <span className="text-xs text-muted-foreground">-</span>)}</TableCell>
                <TableCell className="text-xs">{op.title}</TableCell>
                <TableCell className="font-mono text-xs">{op.summary || '-'}</TableCell>
                <TableCell className="font-mono text-xs">{(op.roles ?? []).join(', ') || '-'}</TableCell>
                <TableCell className={`${STICKY_RIGHT} text-right`}>
                  <Button size="sm" variant="ghost" onClick={() => setEditing(fromOpener(op))}>
                    编辑
                  </Button>
                  <Button size="sm" variant="ghost" disabled={del.isPending} onClick={() => setDeleting(op.name)}>
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
        title="删除 opener"
        message={`确定删除「${deleting}」吗？该操作立即生效且不可恢复。`}
        confirmText="删除"
        danger
        onConfirm={() => {
          if (deleting) del.mutate({ name: deleting });
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />
      {editing && (
        <OpenerForm
          key={editing === EMPTY_DRAFT ? 'new' : `edit:${editing.name}`}
          draft={editing}
          onClose={() => setEditing(null)}
        />
      )}
    </section>
  );
}
