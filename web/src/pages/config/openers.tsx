// 打开工具（openers）管理区块：列表 + 新增/编辑表单（Sheet）+ 删除。
// 数据源是 /api/opener/list（settings.json），保存即生效——config 页其余区块仍读 config.json。
import { useState } from 'react';

import type { Opener } from '@/api/client';
import { ErrorBanner } from '@/components/error-banner';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useIconExtract, useOpenerDelete, useOpenerSave } from '@/queries/opener';
import { useOpenerList } from '@/queries/project';

const ALL_ROLES = ['open-dir', 'open-file', 'diff-dir', 'diff-file'] as const;

// 表单草稿：cmd 以空格分隔编辑（v1 约定：cmd 参数不含空格）
interface Draft {
  name: string;
  cmd: string;
  roles: string[];
  iconType: '' | 'lucide' | 'image';
  iconValue: string;
}

const EMPTY_DRAFT: Draft = {
  name: '',
  cmd: '',
  roles: ['open-dir'],
  iconType: '',
  iconValue: '',
};

function fromOpener(op: Opener): Draft {
  return {
    name: op.name,
    // summary 是命令模板的空格拼接展示，直接还原成编辑文本
    cmd: op.summary,
    roles: op.roles ?? [],
    iconType: op.icon?.type === 'lucide' || op.icon?.type === 'image' ? op.icon.type : '',
    iconValue: op.icon?.value ?? '',
  };
}

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
            <span className="text-xs text-muted-foreground">
              cmd（空格分隔，$0/$1 占位路径槽位；打开工作台可用 `cube web ui $0`）
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
            <span className="text-xs text-muted-foreground">icon（可选）</span>
            <div className="flex gap-2">
              {(['', 'lucide', 'image'] as const).map((t) => (
                <Button
                  key={t || 'none'}
                  size="sm"
                  variant={form.iconType === t ? 'default' : 'outline'}
                  onClick={() => {
                    set('iconType', t);
                    set('iconValue', '');
                  }}
                >
                  {t === '' ? '无' : t}
                </Button>
              ))}
            </div>
            {form.iconType === 'lucide' && (
              <Input
                value={form.iconValue}
                onChange={(e) => set('iconValue', e.target.value)}
                placeholder="folder-open"
              />
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

export function OpenersSection() {
  const openers = useOpenerList();
  const del = useOpenerDelete();
  const [editing, setEditing] = useState<Draft | null>(null);

  return (
    <section>
      <div className="mb-2 flex items-baseline gap-2">
        <h2 className="text-sm font-medium">打开工具（openers）</h2>
        <span className="text-xs text-muted-foreground">settings.json，保存即生效</span>
        <Button size="sm" variant="outline" className="ml-auto" onClick={() => setEditing(EMPTY_DRAFT)}>
          新增
        </Button>
      </div>
      {openers.error && <ErrorBanner message={`加载失败：${openers.error.message}`} />}
      {del.error && <ErrorBanner message={`删除失败：${del.error.message}`} />}
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>name</TableHead>
              <TableHead>cmd</TableHead>
              <TableHead>roles</TableHead>
              <TableHead>icon</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(openers.data?.list ?? []).length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-xs text-muted-foreground">
                  暂无 opener
                </TableCell>
              </TableRow>
            )}
            {(openers.data?.list ?? []).map((op) => (
              <TableRow key={op.name}>
                <TableCell className="font-medium">{op.name}</TableCell>
                <TableCell className="font-mono text-xs">{op.summary || '-'}</TableCell>
                <TableCell className="font-mono text-xs">{(op.roles ?? []).join(', ') || '-'}</TableCell>
                <TableCell className="font-mono text-xs">
                  {op.icon ? `${op.icon.type}:${op.icon.value.slice(0, 12)}` : '-'}
                </TableCell>
                <TableCell className="text-right">
                  <Button size="sm" variant="ghost" onClick={() => setEditing(fromOpener(op))}>
                    编辑
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    disabled={del.isPending}
                    onClick={() => del.mutate({ name: op.name })}
                  >
                    删除
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
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
