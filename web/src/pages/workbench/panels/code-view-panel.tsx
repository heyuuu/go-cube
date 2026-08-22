import { useQueryClient } from '@tanstack/react-query';
import { Pencil, RotateCcw } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';

import { CodeEditor } from '@/components/code-editor';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { ErrorBanner } from '@/components/error-banner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { saveWorkbenchFile, useWorkbenchChanges, useWorkbenchFile, useWorkbenchRefs } from '@/queries/workbench';

import { refShortName, selectSource, sourceLabel, type TreeSource, type WorkbenchParams } from '../params';
import { PanelSplitter } from '../splitter';

import { FileTree } from './file-tree';

// 代码阅读面板（提案 1012）：单选 TreeSource 的文件浏览。
// 虚拟树（commit/ref）只读；真实树（worktree）支持确认式轻编辑——开启编辑、
// 保存各一次弹窗确认；有未保存改动时切换文件/源被阻断。
// 当前浏览的 file 进 URL（刷新恢复），源切换下拉是快捷方式（改写 source 参数）。
//
// 编辑态建模：editState 带「源+文件」键，源/文件变化后旧 editState 自动失效
// （guardSwitch 已阻断带未保存改动的切换，此处只兜底直接改 URL 的场景），
// 不用 effect 重置（React Compiler 禁止 effect 内同步 setState）。

type EditState = { key: string; draft: string };

// 文件树偏好（视图模式/差异范围/宽度）的 localStorage 键
const TREE_VIEW_KEY = 'cube.workbench.codetree.view';
const TREE_SCOPE_KEY = 'cube.workbench.codetree.scope';
const TREE_WIDTH_KEY = 'cube.workbench.codetree.width';

export function CodeViewPanel({ params }: { params: WorkbenchParams }) {
  const { path, source } = params;
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const file = searchParams.get('file') ?? '';

  const [editState, setEditState] = useState<EditState | null>(null);
  const [treeMode, setTreeMode] = useState<'all' | 'diff'>(() =>
    localStorage.getItem(TREE_SCOPE_KEY) === 'diff' ? 'diff' : 'all',
  );
  const [treeView, setTreeView] = useState<'tree' | 'flat'>(() =>
    localStorage.getItem(TREE_VIEW_KEY) === 'flat' ? 'flat' : 'tree',
  );
  // 文件树宽度：个人偏好，存 localStorage（同面板布局），拖拽范围 160~640
  const [treeWidth, setTreeWidth] = useState(() => {
    const v = Number(localStorage.getItem(TREE_WIDTH_KEY));
    return Number.isFinite(v) && v >= 160 && v <= 640 ? v : 240;
  });

  useEffect(() => {
    localStorage.setItem(TREE_VIEW_KEY, treeView);
  }, [treeView]);
  useEffect(() => {
    localStorage.setItem(TREE_SCOPE_KEY, treeMode);
  }, [treeMode]);
  useEffect(() => {
    localStorage.setItem(TREE_WIDTH_KEY, String(treeWidth));
  }, [treeWidth]);

  const [confirmEdit, setConfirmEdit] = useState(false);
  const [confirmSave, setConfirmSave] = useState(false);
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [saving, setSaving] = useState(false);

  const content = useWorkbenchFile(path, source!, file);
  const refs = useWorkbenchRefs(path);
  // changes 全量模式也拉：行级统计（+N -N/琥珀色）在两种模式下都展示
  const changes = useWorkbenchChanges(path, source!, true);
  const fileContent = content.data?.content ?? '';

  const currentKey = `${source?.type}:${source?.id}:${file}`;
  const diffFilter = treeMode === 'diff' && changes.data ? new Set((changes.data.list ?? []).map((e) => e.path)) : null;
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

  const setFileParam = (f: string) => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.set('file', f);
        return next;
      },
      { replace: true },
    );
  };

  const pickFile = (f: string) => guardSwitch(() => setFileParam(f));

  const switchSource = (src: TreeSource) =>
    guardSwitch(() =>
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          selectSource(next, src);
          next.delete('file');
          return next;
        },
        { replace: true },
      ),
    );

  const doSave = async () => {
    setSaving(true);
    setSaveError('');
    try {
      await saveWorkbenchFile(path, source!, file, draft);
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

  return (
    <div className="flex h-full min-h-0">
      <div className="shrink-0 overflow-y-auto border-r border-border" style={{ width: treeWidth }}>
        <FileTree
          path={path}
          source={source}
          selectedFile={file}
          onPick={pickFile}
          filter={diffFilter}
          stats={diffStats}
          statsPending={changes.isPending}
          viewMode={treeView}
          onViewMode={setTreeView}
          scope={treeMode}
          onScope={setTreeMode}
          scopePending={changes.isPending}
        />
      </div>
      <PanelSplitter onDelta={(dx) => setTreeWidth((w) => Math.min(640, Math.max(160, w + dx)))} />
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex shrink-0 items-center gap-2 border-b border-border px-3 py-1.5">
          <span className="truncate text-xs font-medium">{file || '未选择文件'}</span>
          <Badge variant="secondary">{sourceLabel(source)}</Badge>
          {content.data?.binary ? <Badge variant="outline">二进制 {content.data.size}B</Badge> : null}
          {dirty ? <Badge variant="destructive">未保存</Badge> : null}
          <div className="ml-auto flex items-center gap-1.5">
            <SourceSwitcher
              locals={refs.data?.locals ?? []}
              current={source.type === 'ref' ? source.id : ''}
              disabled={dirty}
              onPick={switchSource}
            />
            {canEdit && !editing ? (
              <Button
                variant="outline"
                size="sm"
                disabled={!file || content.data?.binary}
                onClick={() => setConfirmEdit(true)}
              >
                <Pencil className="mr-1 size-3" />
                编辑
              </Button>
            ) : null}
            {editing ? (
              <>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() =>
                    guardSwitch(() => {
                      setEditState(null);
                    })
                  }
                >
                  <RotateCcw className="mr-1 size-3" />
                  取消
                </Button>
                <Button size="sm" disabled={!dirty || saving} onClick={() => setConfirmSave(true)}>
                  保存
                </Button>
              </>
            ) : null}
          </div>
        </div>
        {content.isError ? <ErrorBanner message={content.error.message} /> : null}
        {saveError ? <ErrorBanner message={saveError} /> : null}
        <div className="min-h-0 flex-1 overflow-hidden">
          {!file ? (
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
              key={file}
              value={editing ? draft : fileContent}
              file={file}
              readOnly={!editing}
              onChange={(next) => {
                setEditState((s) => (s ? { ...s, draft: next } : s));
              }}
              className="h-full"
            />
          )}
        </div>
      </div>

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
    </div>
  );
}

// 源切换下拉（快捷方式，非主流程）：直接改写 source 参数，等价于 git 树面板重新选择
function SourceSwitcher({
  locals,
  current,
  disabled,
  onPick,
}: {
  locals: string[];
  current: string;
  disabled: boolean;
  onPick: (src: TreeSource) => void;
}) {
  return (
    <select
      className="h-7 rounded-md border border-border bg-background px-1.5 text-xs text-muted-foreground disabled:opacity-50"
      value={current}
      disabled={disabled}
      onChange={(e) => e.target.value && onPick({ type: 'ref', id: e.target.value })}
    >
      <option value="">切换分支…</option>
      {locals.map((b) => (
        <option key={b} value={b}>
          {refShortName(b)}
        </option>
      ))}
    </select>
  );
}
