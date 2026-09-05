import { useQueryClient } from '@tanstack/react-query';
import { ChevronDown } from 'lucide-react';
import type { UseQueryResult } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';

import type { components } from '@/api/schema';
import { CodeEditor } from '@/components/code-editor';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { ErrorBanner } from '@/components/error-banner';
import { loadMdTheme, mdThemeCls, mdThemes, MarkdownView, type MdThemeId } from '@/components/markdown-render';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Switch } from '@/components/ui/switch';
import { splitInlineDiff, type InlineSegment } from '@/lib/inline-diff';
import { cn } from '@/lib/utils';
import { saveWorkbenchFile } from '@/queries/workbench';
import type { FileDiffResult, FileResult } from '@/queries/workbench';

import type { TreeSource } from '../params';

// 内容区公共组件（code / diff 面板共用）：预览 / 源码 / diff 三模式可切换。
// 源码 = CodeMirror（worktree 源可确认式轻编辑），diff = side-by-side 双栏 hunks，
// 预览 = 按扩展名的富预览（md 渲染 / html iframe / 图片），无对应预览格式的文件
// 等同源码模式（含编辑能力）。
//
// 标题栏只放模式按钮组（元素不随模式增减，切换时布局稳定）；
// 源码模式的编辑开关/保存浮动在代码区右上角，diff 恒显示行号。
//
// 编辑态建模：editState 带「源+文件」键，源/文件变化后旧 editState 自动失效
// （guardSwitch 已阻断带未保存改动的切换，此处只兜底直接改 URL 的场景），
// 不用 effect 重置（React Compiler 禁止 effect 内同步 setState）。

export type ContentMode = 'preview' | 'source' | 'diff';

// 文件形态按扩展名判定（图片/已知二进制不读内容）；null = 文本，读内容渲染
type PreviewKind = 'md' | 'html' | 'image' | 'binary' | null;
const IMAGE_EXTS = ['.png', '.jpg', '.jpeg', '.gif', '.webp', '.svg', '.bmp', '.ico'];
const BINARY_EXTS = [
  '.zip', '.tar', '.gz', '.bz2', '.xz', '.rar', '.7z',
  '.pdf', '.psd', '.ai', '.sketch',
  '.woff', '.woff2', '.ttf', '.otf', '.eot',
  '.exe', '.dll', '.so', '.dylib', '.class', '.jar', '.wasm',
  '.sqlite', '.db',
  '.mp3', '.wav', '.flac', '.ogg', '.mp4', '.mov', '.avi', '.webm',
  '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx',
];
export function previewKindOf(file: string): PreviewKind {
  const dot = file.lastIndexOf('.');
  if (dot < 0) return null;
  const ext = file.slice(dot).toLowerCase();
  if (IMAGE_EXTS.includes(ext)) return 'image';
  if (BINARY_EXTS.includes(ext)) return 'binary';
  if (ext === '.md' || ext === '.markdown') return 'md';
  if (ext === '.html' || ext === '.htm') return 'html';
  return null;
}

// md 预览主题持久化键（与 /md 页的主题偏好各自独立，浏览场景不同）
const MD_THEME_KEY = 'cube.workbench.content.mdtheme';

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
  rawFileUrl,
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
  rawFileUrl: string; // /api/workbench/file/raw 地址（图片预览；面板从 path/src/file 组装）
  headerLeading: ReactNode; // 面板专属徽标（源徽标/文件名等）
}) {
  const fileContent = contentQuery.data?.content ?? '';
  const { editing: isEditing, draft, dirty } = editing;
  const [mdTheme, setMdTheme] = useState<MdThemeId>(() => loadMdTheme(MD_THEME_KEY));
  const switchMdTheme = (t: MdThemeId) => {
    setMdTheme(t);
    localStorage.setItem(MD_THEME_KEY, t);
  };

  const kind = previewKindOf(file);
  const previewKind = mode === 'preview' ? kind : null;
  // 图片预览/源码模式一致（都渲染图片）；已知二进制（扩展名或后端检测结果）两模式都只显示话术
  const isImage = kind === 'image';
  const isBinaryFile = kind === 'binary' || !!contentQuery.data?.binary;
  // 实际渲染源码编辑器的形态：源码模式 + 无富预览回落（编辑能力随之保留）
  const isSourceView = mode === 'source' && !isImage && !isBinaryFile;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 items-center gap-2 border-b border-border px-3 py-1.5 text-xs">
        {headerLeading}
        {mode !== 'diff' && fileMissing ? (
          <Badge variant="outline" className="shrink-0 text-amber-600 dark:text-amber-400">
            原文件不存在，已回退
          </Badge>
        ) : null}
        {mode !== 'diff' && contentQuery.data?.deleted ? (
          <Badge variant="outline" className="shrink-0 text-red-600 dark:text-red-400">
            已删除
          </Badge>
        ) : null}
        {mode !== 'diff' && isBinaryFile ? (
          <Badge variant="outline">二进制{contentQuery.data ? ` ${contentQuery.data.size}B` : ''}</Badge>
        ) : null}
        {isSourceView && dirty ? <Badge variant="destructive">未保存</Badge> : null}
        <div className="ml-auto flex items-center gap-1.5">
          <div className="flex overflow-hidden rounded-md border border-border text-[10px]">
            {(
              [
                ['preview', '预览'],
                ['source', '源码'],
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
                title={
                  m === 'preview'
                    ? '富预览（md 渲染 / html 页面 / 图片；其余格式等同源码）'
                    : m === 'source'
                      ? '源码内容（worktree 源可编辑）'
                      : '与基准/另一源的行级对比'
                }
              >
                {label}
              </button>
            ))}
          </div>
        </div>
      </div>
      {contentQuery.isError && mode !== 'diff' ? <ErrorBanner message={contentQuery.error.message} /> : null}
      {editing.saveError ? <ErrorBanner message={editing.saveError} /> : null}
      <div className="min-h-0 flex-1 overflow-hidden">
        {mode === 'diff' ? (
          !file ? (
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
          )
        ) : !file ? (
          <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
            在左侧选择一个文件
          </div>
        ) : contentQuery.isPending && contentQuery.fetchStatus === 'fetching' ? (
          <div className="p-3 text-xs text-muted-foreground">读取中…</div>
        ) : contentQuery.data?.deleted ? (
          <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
            该文件已从工作区删除（diff 模式可查看删除前的内容）
          </div>
        ) : isImage ? (
          // 图片预览：直连 raw 端点取原始字节（文本 API 对二进制只给标记不给内容）
          <div className="flex h-full items-center justify-center overflow-auto bg-muted/30 p-4">
            <img src={rawFileUrl} alt={file} className="max-h-full max-w-full object-contain" />
          </div>
        ) : previewKind === 'html' ? (
          // sandbox 只放开脚本：允许 AI 生成页交互，不带 allow-same-origin 防读本地文件/cookie
          <iframe title={file} srcDoc={fileContent} sandbox="allow-scripts" className="h-full w-full border-0" />
        ) : previewKind === 'md' ? (
          // 主题选择固定顶部不随滚动；正文单独滚动
          <div className="flex h-full min-h-0 flex-col">
            <div className="flex shrink-0 justify-end border-b border-border px-2 py-1">
              <DropdownMenu>
                <DropdownMenuTrigger render={<Button variant="ghost" size="sm" className="h-6 px-2 text-xs" />}>
                  主题：{mdThemes.find((t) => t.id === mdTheme)?.label ?? mdTheme}
                  <ChevronDown className="size-3" />
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  {mdThemes.map((t) => (
                    <DropdownMenuItem key={t.id} onClick={() => switchMdTheme(t.id)}>
                      {t.label}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
            <div className="min-h-0 flex-1 overflow-auto p-4">
              <MarkdownView content={fileContent} className={mdThemeCls(mdTheme)} />
            </div>
          </div>
        ) : isBinaryFile ? (
          <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
            二进制文件不支持预览{contentQuery.data ? `（${contentQuery.data.size} 字节）` : ''}
          </div>
        ) : (
          // 编辑开关/保存浮动在代码区右上角（不占标题栏，标题栏元素随模式稳定）
          <div className="relative h-full">
            {canEdit ? (
              <div className="absolute top-2 right-3 z-10 flex items-center gap-2 rounded-md border border-border bg-background/80 px-2 py-1 text-xs backdrop-blur-sm">
                <span className="text-muted-foreground" title="预览 / 编辑切换">
                  <span className={cn(!isEditing && 'font-medium text-foreground')}>预览</span>
                  <Switch
                    checked={isEditing}
                    disabled={!file || !!contentQuery.data?.binary || !!contentQuery.data?.deleted}
                    aria-label="切换预览/编辑"
                    onCheckedChange={(checked) => editing.requestEdit(checked)}
                  />
                  <span className={cn(isEditing && 'font-medium text-foreground')}>编辑</span>
                </span>
                {isEditing ? (
                  <Button size="sm" className="h-6 px-2 text-xs" disabled={!dirty || editing.saving} onClick={() => editing.setConfirmSave(true)}>
                    保存
                  </Button>
                ) : null}
              </div>
            ) : null}
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
          </div>
        )}
      </div>
      {editing.dialogs}
    </div>
  );
}

// side-by-side 双栏渲染：del 进左栏、add 进右栏、ctx 两侧同步；连续 del/add 块按行配对。
// 恒显示行号（长行自动换行后靠行号区分行边界）
function SideBySideHunks({ hunks }: { hunks: components['schemas']['Hunk'][] }) {
  if (hunks.length === 0) {
    return <div className="flex flex-1 items-center justify-center text-xs text-muted-foreground">两侧内容一致</div>;
  }
  return (
    <div className="h-full min-h-0 overflow-auto font-mono text-[12px] leading-5">
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
  // 连续 del 块与 add 块逐行配对（左删右增同行对照）为 mod 行，剩余各自单侧展示；
  // 同时按 hunk 起始行推算每行的 old/new 行号（del 只占 old、add 只占 new）
  type Row = { left?: string; right?: string; kind: 'del' | 'add' | 'mod' | 'ctx'; oldNo?: number; newNo?: number };
  const rows: Row[] = [];
  let oldNo = hunk.oldStart;
  let newNo = hunk.newStart;
  let pendingDels: string[] = [];
  const flush = () => {
    for (const d of pendingDels) {
      rows.push({ left: d, kind: 'del', oldNo });
      oldNo++;
    }
    pendingDels = [];
  };
  for (const line of hunk.lines ?? []) {
    if (line.kind === 'ctx') {
      flush();
      rows.push({ left: line.text, right: line.text, kind: 'ctx', oldNo, newNo });
      oldNo++;
      newNo++;
    } else if (line.kind === 'del') {
      pendingDels.push(line.text);
    } else {
      const paired = pendingDels.shift();
      if (paired !== undefined) {
        rows.push({ left: paired, right: line.text, kind: 'mod', oldNo, newNo });
        oldNo++;
        newNo++;
      } else {
        rows.push({ right: line.text, kind: 'add', newNo });
        newNo++;
      }
    }
  }
  flush();

  return (
    <table className="w-full table-fixed border-collapse">
      <tbody>
        {rows.map((r, i) => {
          // mod 行整行淡底、行内仅差异字符换字体色（Beyond Compare 风格）；
          // 纯增删行整行高亮语义不变
          const leftChanged = r.kind === 'del' || r.kind === 'mod';
          const rightChanged = r.kind === 'add' || r.kind === 'mod';
          const inline = r.kind === 'mod' ? splitInlineDiff(r.left ?? '', r.right ?? '') : null;
          const gutterCls = 'w-10 select-none text-right align-top text-[10px] leading-5 text-muted-foreground/60';
          return (
            <tr key={i}>
              <td className={gutterCls}>{r.oldNo ?? ''}</td>
              <td
                className={cn(
                  'whitespace-pre-wrap break-all border-r border-border px-2 align-top',
                  leftChanged && (r.kind === 'mod' ? 'bg-red-500/10' : 'bg-red-500/10 text-red-600 dark:text-red-400'),
                )}
              >
                {inline ? <InlineSegments segs={inline.left} tone="del" /> : (r.left ?? '')}
              </td>
              <td className={gutterCls}>{r.newNo ?? ''}</td>
              <td
                className={cn(
                  'whitespace-pre-wrap break-all px-2 align-top',
                  rightChanged &&
                    (r.kind === 'mod'
                      ? 'bg-emerald-500/10'
                      : 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'),
                )}
              >
                {inline ? <InlineSegments segs={inline.right} tone="add" /> : (r.right ?? '')}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

// mod 行行内片段渲染：changed 片段仅换字体色（底色由整行 <td> 提供），公共片段保持原字体色
function InlineSegments({ segs, tone }: { segs: InlineSegment[]; tone: 'del' | 'add' }) {
  const changedCls = tone === 'del' ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400';
  return (
    <>
      {segs.map((s, i) =>
        s.changed ? (
          <span key={i} className={changedCls}>
            {s.text}
          </span>
        ) : (
          s.text
        ),
      )}
    </>
  );
}
