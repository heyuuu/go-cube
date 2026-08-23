import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useSearchParams } from 'react-router';

import { CodeEditor } from '@/components/code-editor';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { ErrorBanner } from '@/components/error-banner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import {
  saveWorkbenchFile,
  useWorkbenchChanges,
  useWorkbenchFile,
  useWorkbenchTree,
} from '@/queries/workbench';

import { cn } from '@/lib/utils';
import { sourceLabel, writeFileParam, type WorkbenchParams } from '../params';

import { SourcePanelShell, useTreePanePrefs } from './tree-pane';

// 代码阅读面板（提案 1012）：单选 TreeSource 的文件浏览。
// 虚拟树（commit/ref）只读；真实树（worktree）支持确认式轻编辑——开启编辑、
// 保存各一次弹窗确认；有未保存改动时切换文件/源被阻断。
// 当前浏览的 file 进 URL（刷新恢复）；源只展示不切换——选择入口统一在 git 树面板，
// 面板内再做切换会与之冲突（早期版本的下拉可选项也不全，已移除）。
//
// 编辑态建模：editState 带「源+文件」键，源/文件变化后旧 editState 自动失效
// （guardSwitch 已阻断带未保存改动的切换，此处只兜底直接改 URL 的场景），
// 不用 effect 重置（React Compiler 禁止 effect 内同步 setState）。

type EditState = { key: string; draft: string };

// 文件树偏好键前缀（视图/范围/宽度，见 tree-pane.tsx 的 useTreePanePrefs）
const TREE_PREFS_KEY = 'cube.workbench.codetree';

export function CodeViewPanel({ params }: { params: WorkbenchParams }) {
  const { path, source } = params;
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const file = searchParams.get('file') ?? '';

  const [editState, setEditState] = useState<EditState | null>(null);
  const treePrefs = useTreePanePrefs(TREE_PREFS_KEY, 'all');

  const [confirmEdit, setConfirmEdit] = useState(false);
  const [confirmSave, setConfirmSave] = useState(false);
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [saving, setSaving] = useState(false);

  // 与 FileTree 同 key 的树数据（react-query 去重复用，无额外请求）：
  // 用于回退判定——切换 commit/分支后 file 参数可能指向新源里不存在的文件
  const tree = useWorkbenchTree(path, source!);
  const treeList = tree.data?.list ?? null;
  // 回退策略：file 不在当前源中 → 根目录 README.md → 都没有则空并提示。
  // 只做显示层回退不改写 URL——file 参数指向用户最后的选择，源切换是临时浏览上下文
  const fileMissing = !!treeList && !!file && !treeList.includes(file);
  const activeFile = !fileMissing ? file : treeList?.includes('README.md') ? 'README.md' : '';

  const content = useWorkbenchFile(path, source!, activeFile);
  // changes 全量模式也拉：行级统计（+N -N/琥珀色）在两种模式下都展示
  const changes = useWorkbenchChanges(path, source!, true);
  const fileContent = content.data?.content ?? '';

  const currentKey = `${source?.type}:${source?.id}:${activeFile}`;
  const diffFilter =
    treePrefs.scope === 'diff' && changes.data ? new Set((changes.data.list ?? []).map((e) => e.path)) : null;
  // 差异文件的行级增删与状态（后端 Changes 注入），键为文件相对路径
  const diffStats = changes.data
    ? new Map(
        (changes.data.list ?? []).map((e) => [
          e.path,
          { adds: e.adds, dels: e.dels, binary: e.binary, status: e.status, oldPath: e.oldPath || undefined },
        ]),
      )
    : null;
  const editing = editState !== null && editState.key === currentKey;
  const draft = editing ? editState.draft : '';
  const dirty = editing && editState.draft !== fileContent;
  const canEdit = source?.type === 'worktree';

  // 丢弃确认通过后要执行的切换动作（确认弹窗是渲染外的暂存，不用 ref）
  const [pendingAction, setPendingAction] = useState<null | (() => void)>(null);

  // 切换文件 / 源的守卫：未保存改动时先弹丢弃确认
  const guardSwitch = (action: () => void) => {
    if (dirty) {
      setPendingAction(() => action);
      setConfirmDiscard(true);
      return;
    }
    action();
  };

  const pickFile = (f: string) =>
    guardSwitch(() =>
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          writeFileParam(next, f);
          return next;
        },
        { replace: true },
      ),
    );

  const doSave = async () => {
    setSaving(true);
    setSaveError('');
    try {
      await saveWorkbenchFile(path, source!, activeFile, draft);
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

  if (!source) return null;

  const header = (
    <>
      <Badge variant="secondary" className="max-w-48 shrink-0" title={source.id}>
        <span className="text-[10px] text-muted-foreground">
          {source.type === 'worktree' ? '工作副本' : source.type === 'ref' ? '分支' : '提交'}
        </span>
        <span className="ml-1 truncate font-mono">{sourceLabel(source)}</span>
      </Badge>
      <span className="truncate font-medium">{activeFile || '未选择文件'}</span>
      {fileMissing ? (
        <Badge variant="outline" className="shrink-0 text-amber-600 dark:text-amber-400">
          {activeFile ? '原文件不存在，已回退 README.md' : '原文件在此目标中不存在'}
        </Badge>
      ) : null}
      {content.data?.binary ? <Badge variant="outline">二进制 {content.data.size}B</Badge> : null}
      {dirty ? <Badge variant="destructive">未保存</Badge> : null}
      <div className="ml-auto flex items-center gap-1.5">
        {canEdit ? (
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground" title="预览 / 编辑切换">
            <span className={cn(!editing && 'font-medium text-foreground')}>预览</span>
            <Switch
              checked={editing}
              disabled={!activeFile || !!content.data?.binary}
              aria-label="切换预览/编辑"
              onCheckedChange={(checked) => {
                if (checked) {
                  setConfirmEdit(true);
                } else {
                  // 关编辑 = 旧「取消」：有未保存改动走丢弃确认，否则直接退出
                  guardSwitch(() => setEditState(null));
                }
              }}
            />
            <span className={cn(editing && 'font-medium text-foreground')}>编辑</span>
          </div>
        ) : null}
        {editing ? (
          <Button size="sm" disabled={!dirty || saving} onClick={() => setConfirmSave(true)}>
            保存
          </Button>
        ) : null}
      </div>
    </>
  );

  return (
    <SourcePanelShell
      prefs={treePrefs}
      path={path}
      treeSource={source}
      selectedFile={activeFile}
      onPick={pickFile}
      diffFilter={diffFilter}
      stats={diffStats}
      statsPending={changes.isPending}
      header={header}
    >
      {content.isError ? <ErrorBanner message={content.error.message} /> : null}
      {saveError ? <ErrorBanner message={saveError} /> : null}
      {fileMissing && !activeFile ? (
        <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
          当前文件在所选目标中不存在（无 README.md 可回退）
        </div>
      ) : !activeFile ? (
        <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
          在左侧选择一个文件
        </div>
      ) : content.isPending ? (
        <div className="p-3 text-xs text-muted-foreground">读取中…</div>
      ) : content.data?.binary ? (
        <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
          二进制文件不支持预览（{content.data.size} 字节）
        </div>
      ) : (
        <CodeEditor
          key={activeFile}
          value={editing ? draft : fileContent}
          file={activeFile}
          readOnly={!editing}
          onChange={(next) => {
            setEditState((s) => (s ? { ...s, draft: next } : s));
          }}
          className="h-full"
        />
      )}

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
    </SourcePanelShell>
  );
}
