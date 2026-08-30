// settings 页 Opener 分区：列表 + 抽屉（Sheet）编辑表单，删除前确认。
// 数据源是 /api/opener/list（settings.json），保存即生效。
import { ChevronDown, GripVertical } from 'lucide-react';
import { useState } from 'react';

import type { Opener } from '@/api/client';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { ErrorBanner } from '@/components/error-banner';
import { IconField } from '@/components/icon-field';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import type { IconDecl } from '@/lib/icon';
import { renderIcon } from '@/lib/icon';
import { cn } from '@/lib/utils';
import { useIntentDefaultDelete, useIntentDefaultSave, useOpenerDelete, useOpenerReorder, useOpenerSave, useOpenerIntents } from '@/queries/opener';
import { useOpenerList } from '@/queries/project';

const ALL_ROLES = ['open-dir', 'open-file', 'diff-dir', 'diff-file'] as const;

const PLACEHOLDER_BY_ROLE: Record<(typeof ALL_ROLES)[number], string> = {
  'open-dir': 'exec: code $0',
  'open-file': 'exec: code $0',
  'diff-dir': 'exec: bcompare $0 $1',
  'diff-file': 'exec: code --diff $0 $1',
};

// 冻结列：name 贴左、操作贴右，水平滚动时始终可见。实心 bg 遮住下层滑过的单元格，
// 分隔线用 inset 阴影而非 border（border-collapse 下 border 不随 sticky 单元格移动）；
// 行 hover 靠 group 保持整行联动（sticky 单元格自身的 bg 会盖掉 tr 的 hover bg）
const STICKY_LEFT =
  'sticky left-0 z-10 bg-background group-hover/row:bg-muted/50 shadow-[inset_-1px_0_0_var(--border)]';
const STICKY_RIGHT =
  'sticky right-0 z-10 bg-background group-hover/row:bg-muted/50 shadow-[inset_1px_0_0_var(--border)]';

// 与后端 defaultIcon 一致（opener/icon.go）：icon 必有值，新增默认 lucide:app-window-mac
const DEFAULT_LUCIDE = 'app-window-mac';

// 表单草稿：actions 按 role 各一条动作串（`<kind>:<模板>`，exec: 命令为 sh 风格——
// 空格分隔，含空格的 token 用引号包裹，如 "/Applications/Visual Studio Code.app/..."）
interface Draft {
  name: string;
  title: string;
  actions: Record<string, string>;
  icon: IconDecl;
}

const EMPTY_DRAFT: Draft = {
  name: '',
  title: '',
  actions: { 'open-dir': '' },
  icon: { type: 'lucide', value: DEFAULT_LUCIDE },
};

function fromOpener(op: Opener): Draft {
  return {
    name: op.name,
    title: op.title === `用 ${op.name} 打开` ? '' : op.title,
    actions: { ...(op.actions ?? {}) },
    icon:
      op.icon?.type === 'image'
        ? { type: 'image', value: op.icon.value ?? '' }
        : { type: 'lucide', value: op.icon?.value || DEFAULT_LUCIDE },
  };
}

// 编辑表单（右侧抽屉）：关闭即放弃草稿，保存成功后自动关闭
function OpenerForm({ draft, onClose }: { draft: Draft; onClose: () => void }) {
  const [form, setForm] = useState<Draft>(draft);
  const save = useOpenerSave();
  const set = <K extends keyof Draft>(key: K, value: Draft[K]) => setForm((f) => ({ ...f, [key]: value }));

  const setAction = (role: string, raw: string) =>
    setForm((f) => ({ ...f, actions: { ...f.actions, [role]: raw } }));
  const toggleRole = (role: string, on: boolean) =>
    setForm((f) => {
      const actions = { ...f.actions };
      if (on) actions[role] = actions[role] ?? '';
      else delete actions[role];
      return { ...f, actions };
    });

  const submit = () => {
    // 过滤空动作串的 role（后端对空模板报错；勾了没填=未声明）
    const actions = Object.fromEntries(Object.entries(form.actions).filter(([, raw]) => raw.trim() !== ''));
    save.mutate(
      {
        name: form.name.trim(),
        title: form.title.trim() || undefined,
        actions,
        icon: { type: form.icon.type, value: form.icon.value },
      },
      { onSuccess: onClose },
    );
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

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">name（唯一标识）</span>
            <Input value={form.name} onChange={(e) => set('name', e.target.value)} placeholder="code" />
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">title（展示文案，缺省生成「用 name 打开」）</span>
            <Input value={form.title} onChange={(e) => set('title', e.target.value)} placeholder="打开所在目录" />
          </label>

          <div className="flex flex-col gap-2">
            <span className="text-xs text-muted-foreground">
              actions（每个用途一条动作串：exec: 命令 / url: 链接；$0/$1 占位路径槽位，站内路由如 url: /workbench?path=$0）
            </span>
            {ALL_ROLES.map((r) => (
              <div key={r} className="flex items-center gap-2">
                <label className="flex items-center gap-1.5 text-xs">
                  <Checkbox
                    checked={r in form.actions}
                    onCheckedChange={(v) => toggleRole(r, v === true)}
                  />
                  {r}
                </label>
                {r in form.actions && (
                  <Input
                    value={form.actions[r]}
                    onChange={(e) => setAction(r, e.target.value)}
                    placeholder={PLACEHOLDER_BY_ROLE[r]}
                    className="font-mono"
                  />
                )}
              </div>
            ))}
          </div>

          <div className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">icon（必有值，未选择时用默认图）</span>
            <IconField
              value={form.icon}
              onChange={(v) => set('icon', v ?? { type: 'lucide', value: DEFAULT_LUCIDE })}
              defaultLucide={DEFAULT_LUCIDE}
            />
          </div>

          <div className="mt-2 flex justify-end gap-2">
            <Button size="sm" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button size="sm" disabled={save.isPending || !form.name.trim() || Object.keys(form.actions).length === 0} onClick={submit}>
              保存
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}

// 打开意图分区：每个 intent 一行「默认 opener」下拉（候选 = 该 intent 的 openers）。
// 选「无默认」即删除；保存即生效。候选/默认的失效清理由后端读侧完成，此处只见有效值。
function IntentSection() {
  const intents = useOpenerIntents();
  const save = useIntentDefaultSave();
  const del = useIntentDefaultDelete();

  const onChange = (intent: string, opener: string) => {
    if (opener === '') del.mutate({ intent });
    else save.mutate({ intent, opener });
  };

  return (
    <div className="mt-6">
      <div className="mb-2 flex items-baseline gap-2">
        <h2 className="text-sm font-medium">打开意图（intents）</h2>
        <span className="text-xs text-muted-foreground">每个场景一个默认 opener；CLI 不传 -o 时直接使用默认</span>
      </div>
      {intents.error && <ErrorBanner message={`加载失败：${intents.error.message}`} />}
      {(save.error || del.error) && (
        <ErrorBanner message={`保存失败：${((save.error || del.error) as Error).message}`} />
      )}
      <div className="flex flex-col divide-y rounded-lg border">
        {(intents.data ?? []).map((it) => (
          <label
            key={it.intent}
            className="grid grid-cols-[6rem_14rem_1fr] items-center gap-3 px-3 py-1.5 text-sm"
          >
            <span className="font-mono text-xs/relaxed">{it.intent}</span>
            <div className="relative">
              <select
                className="h-7 w-full appearance-none rounded-md border border-input bg-input/20 px-2 pr-7 text-xs/relaxed transition-colors outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/30 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30"
                value={it.defaultOpener ?? ''}
                disabled={save.isPending || del.isPending}
                onChange={(e) => onChange(it.intent, e.target.value)}
              >
                <option value="">无默认</option>
                {it.openers.map((name) => (
                  <option key={name} value={name}>
                    {name}
                  </option>
                ))}
              </select>
              <ChevronDown className="pointer-events-none absolute right-2 top-1/2 size-3 -translate-y-1/2 text-muted-foreground" />
            </div>
            <span className="text-xs/relaxed text-muted-foreground">
              {it.defaultOpener ? '' : '未配置默认'}
              {it.openers.length} 个候选
            </span>
          </label>
        ))}
      </div>
    </div>
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
                <TableCell>{renderIcon(op?.icon, <span className="text-xs text-muted-foreground">-</span>)}</TableCell>
                <TableCell className="text-xs">{op.title}</TableCell>
                <TableCell className="font-mono text-xs">{op.summary || '-'}</TableCell>
                <TableCell className="font-mono text-xs">{Object.keys(op.actions ?? {}).join(', ') || '-'}</TableCell>
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
      <IntentSection />
    </section>
  );
}
