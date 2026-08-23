import { useQueryClient } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';

import { CodeEditor } from '@/components/code-editor';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { ErrorBanner } from '@/components/error-banner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import type { components } from '@/api/schema';
import { saveWorkbenchFile } from '@/queries/workbench';
import type { FileDiffResult, FileResult } from '@/queries/workbench';
import { cn } from '@/lib/utils';

import type { UseQueryResult } from '@tanstack/react-query';

import type { TreeSource } from '../params';

// 内容区公共组件（code / diff 面板共用）：单文件模式（CodeMirror，worktree 源可确认式轻编辑）
// 与 diff 模式（side-by-side 双栏 hunks）可切换。code 面板由此获得 diff 展示，
// diff 面板在右侧源为 worktree 时获得编辑能力。
//
// 编辑态建模：editState 带「源+文件」键，源/文件变化后旧 editState 自动失效
// （guardSwitch 已阻断带未保存改动的切换，此处只兜底直接改 URL 的场景），
// 不用 effect 重置（React Compiler 禁止 effect 内同步 setState）。

export type ContentMode = 'file' | 'diff';

type EditState = { key: string; draft: string };

// 确认式轻编辑流（从 code 面板抽出）：开启编辑/保存双弹窗、未保存切换守卫。
// 面板用 guardSwitch 包住文件/源切换动作；dialogs 需渲染到面板树上。
export function useFileEditing(path: string, src: TreeSource, file: string, fileContent: string) {
  const queryClient = useQueryClient();
  const [editState, setEditState] = useState<EditState | null>(null);
  const [confirmEdit, setConfirmEdit] = useState(false);
  const [confirmSave, setConfirmSave] = useState(false);
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [saving, setSaving] = useState(false);
  // 丢弃确认通过后要执行的切换动作（确认弹窗是渲染外的暂存，不用 ref）
  const [pendingAction, setPendingAction] = useState<null | (() => void)>(null);

  const currentKey = `${src.type}:${src.id}:${file}`;
  const editing = editState !== null && editState.key === currentKey;
  const draft = editing ? editState.draft : '';
  const dirty = editing && editState.draft !== fileContent;

  // 切换文件 / 源的守卫：未保存改动时先弹丢弃确认
  const guardSwitch = (action: () => void) => {
    if (dirty) {
      setPendingAction(() => action);
      setConfirmDiscard(true);
      return;
    }
    action();
  };

  const requestEdit = (on: boolean) => {
    if (on) setConfirmEdit(true);
    else guardSwitch(() => setEditState(null)); // 关编辑 = 取消：脏改动先丢弃确认
  };

  const doSave = async () => {
    setSaving(true);
    setSaveError('');
    try {
      await saveWorkbenchFile(path, src, file, draft);
      setEditState(null);
      await queryClient.invalidateQueries({ queryKey: ['workbench', 'file', path] });
      await queryClient.invalidateQueries({ queryKey: ['workbench', 'status', path] });
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : '保存失败');
    } finally {
      setSaving(false);
      setConfirmSave(false);
    }
  };

  const dialogs = (
    <>
      <ConfirmDialog
        open={confirmEdit}
        title="开启编辑"
        message={`将修改工作区文件 ${file}。编辑期间切换文件/分支会被阻断，直到保存或放弃改动。`}
        confirmText="开始编辑"
        onCancel={() => setConfirmEdit(false)}
        onConfirm={() => {
          setEditState({ key: currentKey, draft: fileContent });
          setConfirmEdit(false);
        }}
      />
      <ConfirmDialog
        open={confirmSave}
        title="保存文件"
        message={`确认将改动写入工作区文件 ${file}？此操作只落盘，不执行任何 git 操作。`}
        confirmText="保存"
        onConfirm={doSave}
        onCancel={() => setConfirmSave(false)}
      />
      <ConfirmDialog
        open={confirmDiscard}
        title="放弃未保存的改动"
        message="当前有未保存的编辑，继续切换将丢弃这些改动。"
        confirmText="丢弃并切换"
        danger
        onCancel={() => setConfirmDiscard(false)}
        onConfirm={() => {
          setEditState(null);
          setConfirmDiscard(false);
          const action = pendingAction;
          setPendingAction(null);
          action?.();
        }}
      />
    </>
  );

  const setDraft = (next: string) => setEditState((s) => (s ? { ...s, draft: next } : s));

  return { editing, draft, dirty, saving, saveError, guardSwitch, requestEdit, setConfirmSave, setDraft, dialogs };
}

export type EditingController = ReturnType<typeof useFileEditing>;

export function FileContentArea({
  mode,
  onMode,
  file,
  fileMissing,
  canEdit,
  editing,
  contentQuery,
  fileDiffQuery,
  headerLeading,
}: {
  mode: ContentMode;
  onMode: (m: ContentMode) => void;
  file: string;
  fileMissing?: boolean; // file 不在单文件源中（code 面板的回退判定），显示提示徽标
  canEdit: boolean; // 单文件源是否 worktree（可编辑）
  editing: EditingController;
  contentQuery: UseQueryResult<FileResult>;
  fileDiffQuery: UseQueryResult<FileDiffResult>;
  headerLeading: ReactNode; // 面板专属徽标（源徽标/文件名等）
}) {
  const fileContent = contentQuery.data?.content ?? '';
  const { editing: isEditing, draft, dirty } = editing;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 items-center gap-2 border-b border-border px-3 py-1.5 text-xs">
        {headerLeading}
        {mode === 'file' && fileMissing ? (
          <Badge variant="outline" className="shrink-0 text-amber-600 dark:text-amber-400">
            原文件不存在，已回退
          </Badge>
        ) : null}
        {mode === 'file' && contentQuery.data?.binary ? (
          <Badge variant="outline">二进制 {contentQuery.data.size}B</Badge>
        ) : null}
        {mode === 'file' && dirty ? <Badge variant="destructive">未保存</Badge> : null}
        <div className="ml-auto flex items-center gap-1.5">
          <div className="flex overflow-hidden rounded-md border border-border text-[10px]">
            {(
              [
                ['file', '单文件'],
                ['diff', 'diff'],
              ] as const
            ).map(([m, label]) => (
              <button
                key={m}
                type="button"
                className={cn(
                  'px-1.5 py-0.5 transition-colors',
                  mode === m ? 'bg-primary/15 font-medium text-primary' : 'text-muted-foreground hover:bg-accent',
                )}
                onClick={() => editing.guardSwitch(() => onMode(m))}
                title={m === 'file' ? '单文件内容（worktree 源可编辑）' : '与基准/另一源的行级对比'}
              >
                {label}
              </button>
            ))}
          </div>
          {mode === 'file' && canEdit ? (
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground" title="预览 / 编辑切换">
              <span className={cn(!isEditing && 'font-medium text-foreground')}>预览</span>
              <Switch
                checked={isEditing}
                disabled={!file || !!contentQuery.data?.binary}
                aria-label="切换预览/编辑"
                onCheckedChange={(checked) => editing.requestEdit(checked)}
              />
              <span className={cn(isEditing && 'font-medium text-foreground')}>编辑</span>
            </div>
          ) : null}
          {mode === 'file' && isEditing ? (
            <Button size="sm" disabled={!dirty || editing.saving} onClick={() => editing.setConfirmSave(true)}>
              保存
            </Button>
          ) : null}
        </div>
      </div>
      {contentQuery.isError && mode === 'file' ? <ErrorBanner message={contentQuery.error.message} /> : null}
      {editing.saveError ? <ErrorBanner message={editing.saveError} /> : null}
      <div className="min-h-0 flex-1 overflow-hidden">
        {mode === 'file' ? (
          fileMissing && !file ? (
            <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
              当前文件在所选目标中不存在
            </div>
          ) : !file ? (
            <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
              在左侧选择一个文件
            </div>
          ) : contentQuery.isPending ? (
            <div className="p-3 text-xs text-muted-foreground">读取中…</div>
          ) : contentQuery.data?.binary ? (
            <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
              二进制文件不支持预览（{contentQuery.data.size} 字节）
            </div>
          ) : (
            <CodeEditor
              key={file}
              value={isEditing ? draft : fileContent}
              file={file}
              readOnly={!isEditing}
              onChange={(next) => {
                // editState 只在编辑中存在；onChange 仅编辑态会触发
                editing.setDraft(next);
              }}
              className="h-full"
            />
          )
        ) : !file ? (
          <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
            在左侧选择一个变更文件查看双栏对比
          </div>
        ) : fileDiffQuery.isPending ? (
          <div className="p-3 text-xs text-muted-foreground">计算 diff…</div>
        ) : fileDiffQuery.isError ? (
          <ErrorBanner message={fileDiffQuery.error.message} />
        ) : fileDiffQuery.data?.binary ? (
          <div className="flex h-full items-center justify-center text-xs text-muted-foreground">二进制文件差异</div>
        ) : (
          <SideBySideHunks hunks={fileDiffQuery.data?.hunks ?? []} />
        )}
      </div>
      {editing.dialogs}
    </div>
  );
}

// side-by-side 双栏渲染：del 进左栏、add 进右栏、ctx 两侧同步；连续 del/add 块按行配对
function SideBySideHunks({ hunks }: { hunks: components['schemas']['Hunk'][] }) {
  if (hunks.length === 0) {
    return <div className="flex flex-1 items-center justify-center text-xs text-muted-foreground">两侧内容一致</div>;
  }
  return (
    <div className="min-h-0 flex-1 overflow-auto font-mono text-[12px] leading-5">
      {hunks.map((h, i) => (
        <div key={i}>
          <div className="bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
            @@ -{h.oldStart},{h.oldCount} +{h.newStart},{h.newCount} @@
          </div>
          <HunkRows hunk={h} />
        </div>
      ))}
    </div>
  );
}

function HunkRows({ hunk }: { hunk: components['schemas']['Hunk'] }) {
  // 连续 del 块与 add 块逐行配对（左删右增同行对照），剩余各自单侧展示
  type Row = { left?: string; right?: string; kind: 'del' | 'add' | 'pair' };
  const rows: Row[] = [];
  let pendingDels: string[] = [];
  const flush = () => {
    for (const d of pendingDels) rows.push({ left: d, kind: 'del' });
    pendingDels = [];
  };
  for (const line of hunk.lines ?? []) {
    if (line.kind === 'ctx') {
      flush();
      rows.push({ left: line.text, right: line.text, kind: 'pair' });
    } else if (line.kind === 'del') {
      pendingDels.push(line.text);
    } else {
      const paired = pendingDels.shift();
      if (paired !== undefined) {
        rows.push({ left: paired, right: line.text, kind: 'pair' });
      } else {
        rows.push({ right: line.text, kind: 'add' });
      }
    }
  }
  flush();

  return (
    <table className="w-full table-fixed border-collapse">
      <tbody>
        {rows.map((r, i) => {
          const changed = r.kind !== 'pair';
          const leftChanged = changed && r.left !== undefined && (r.right === undefined || r.kind === 'del');
          const rightChanged = changed && r.right !== undefined && (r.left === undefined || r.kind === 'add');
          return (
            <tr key={i} className="align-top">
              <td
                className={cn(
                  'w-1/2 whitespace-pre-wrap break-all border-r border-border px-2',
                  leftChanged && 'bg-red-500/10 text-red-600 dark:text-red-400',
                )}
              >
                {r.left ?? ''}
              </td>
              <td
                className={cn(
                  'w-1/2 whitespace-pre-wrap break-all px-2',
                  rightChanged && 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
                )}
              >
                {r.right ?? ''}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
