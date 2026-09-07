// git 树面板·工作副本状态区（提案 1011/1031）：worktree 分组（各自分支/ahead-behind/脏状态）、
// 远端配置列表、分支行；副本行的打开动作组（快捷 intent + 全量下拉）。
import {
  Check,
  ChevronRight,
  Cloud,
  Copy,
  ExternalLink,
  Eye,
  EyeOff,
  GitBranch,
  GitCompare,
  Monitor,
  Plus,
  RotateCcw,
  Trash2,
  Ellipsis,
  Eraser,
} from 'lucide-react';
import { useState } from 'react';

import { ErrorBanner } from '@/components/error-banner';
import { CopyPathItem, OpenWithGroup } from '@/components/open-with-menu';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { usePersistentSet } from '@/hooks/use-local-pref';
import { renderIcon } from '@/lib/icon';
import { cn } from '@/lib/utils';
import { filterTargets, quickIntents, type QuickIntent, type TargetKind } from '@/pages/projects/shared';
import { useIntentDefaultOpener, useOpenerList } from '@/queries/opener';
import { useOpenerDiffOpen, useOpenerOpen } from '@/queries/project';
import {
  useWorkbenchRefs,
  useWorkbenchRemotes,
  useWorkbenchWorktrees,
  useWorktreePrune,
  type RemoteEntry,
  type WorktreeStatus,
} from '@/queries/workbench';

import { refShortName, sameSource, type TreeSource, type WorkbenchParams } from '../params';

import { Section, SelectableRow } from './git-tree-bits';
import { WorktreeResetDialog } from './worktree-write';

// workspace 子行展开态持久化（与 worktree-visibility 同款模式）
const WS_EXPANDED_KEY = 'cube.workbench.wsExpanded';

// --- 工作副本状态区 ---

// 去尾部斜杠后比较路径（主根判定用，容忍尾斜杠差异）
const trimSlash = (s: string) => s.replace(/\/+$/, '');

export function WorktreeSection({
  path,
  params,
  onBranchPicked,
  hiddenWorktrees,
  onToggleWorktree,
  onAddWorktree,
  onRemoveWorktree,
  onDeleteBranch,
  onAddBranch,
}: {
  path: string;
  params: WorkbenchParams;
  onBranchPicked: () => void;
  hiddenWorktrees: Set<string>;
  onToggleWorktree: (wtPath: string) => void;
  onAddWorktree: (prefill: { branch?: string; commitish?: string }) => void;
  onRemoveWorktree: (wt: WorktreeStatus) => void;
  onDeleteBranch: (name: string) => void;
  onAddBranch: () => void;
}) {
  const refs = useWorkbenchRefs(path);
  const worktrees = useWorkbenchWorktrees(path);
  // workspace 子行展开态集合在此持有，行组件只读写内存（此前每行渲染期各自
  // JSON.parse 整份 localStorage，行数 × 渲染次重复解析）
  const wsExpanded = usePersistentSet(WS_EXPANDED_KEY);
  // prune 幂等无损，无需确认弹窗；失败就地展示错误
  const prune = useWorktreePrune(path);

  return (
    <>
      <Section
        title="工作副本"
        icon={<Monitor className="size-3.5" />}
        action={
          <div className="flex items-center">
            <Button
              variant="ghost"
              size="icon-sm"
              title="清理失效的 worktree 记录（prune）"
              aria-label="清理失效的 worktree 记录"
              disabled={prune.isPending}
              onClick={() => prune.mutate()}
            >
              <Eraser className="size-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              title="新建 worktree"
              aria-label="新建 worktree"
              onClick={() => onAddWorktree({})}
            >
              <Plus className="size-3.5" />
            </Button>
          </div>
        }
      >
        {prune.isError ? <ErrorBanner message={prune.error.message} /> : null}
        {(worktrees.data ?? []).map((wt) => (
          <WorktreeRow
            key={wt.path}
            wt={wt}
            kind={trimSlash(wt.path) === trimSlash(path) ? 'root' : 'worktree'}
            rootPath={path}
            params={params}
            afterSelect={onBranchPicked}
            hidden={hiddenWorktrees.has(wt.path)}
            onToggle={() => onToggleWorktree(wt.path)}
            onRemove={() => onRemoveWorktree(wt)}
            wsExpanded={wsExpanded.set.has(wt.path)}
            onToggleWs={() => wsExpanded.toggle(wt.path)}
          />
        ))}
      </Section>
      <RemoteSection path={path} />
      <Section
        title="分支"
        icon={<GitBranch className="size-3.5" />}
        action={
          <Button variant="ghost" size="icon-sm" title="新建分支" aria-label="新建分支" onClick={onAddBranch}>
            <Plus className="size-3.5" />
          </Button>
        }
      >
        {(refs.data?.locals ?? []).map((b) => (
          <BranchRow
            key={b}
            refName={b}
            isHead={b === refs.data?.head}
            params={params}
            afterSelect={onBranchPicked}
            onAddWorktree={() => onAddWorktree({ branch: refShortName(b) })}
            onDeleteBranch={() => onDeleteBranch(refShortName(b))}
          />
        ))}
      </Section>
    </>
  );
}

// --- 远端分组：remote 配置列表（非 ref，不可选中），复制地址 + 跳转托管平台网页 ---

function RemoteSection({ path }: { path: string }) {
  const remotes = useWorkbenchRemotes(path);
  const list = remotes.data ?? [];
  if (list.length === 0) return null;
  return (
    <Section title="远端" icon={<Cloud className="size-3.5" />}>
      {list.map((r) => (
        <RemoteRow key={r.name} remote={r} />
      ))}
    </Section>
  );
}

function RemoteRow({ remote }: { remote: RemoteEntry }) {
  const [copied, setCopied] = useState(false);
  const copy = () => {
    void navigator.clipboard.writeText(remote.url).then(() => {
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    });
  };
  return (
    <div className="flex items-center px-2 leading-7 text-xs">
      <span className="shrink-0 font-medium">{remote.name}</span>
      <span className="ml-2 min-w-0 truncate font-mono text-[10px] text-muted-foreground" title={remote.url}>
        {remote.url}
      </span>
      <Button
        variant="ghost"
        size="icon-sm"
        className="ml-auto shrink-0"
        title="复制 git 地址"
        aria-label={`复制 ${remote.name} 的地址`}
        onClick={copy}
      >
        {copied ? <Check className="size-3.5 text-green-600" /> : <Copy className="size-3.5" />}
      </Button>
      {remote.webUrl ? (
        <Button
          variant="ghost"
          size="icon-sm"
          className="shrink-0"
          title={remote.webUrl}
          aria-label={`打开 ${remote.name} 的网页`}
          onClick={() => window.open(remote.webUrl, '_blank', 'noopener')}
        >
          <ExternalLink className="size-3.5" />
        </Button>
      ) : null}
    </div>
  );
}

// 分支行：可选中主体 + 尾部 hover 动作（在此新建 worktree / 删除分支）。
// 与工作副本行同构：动作按钮与 SelectableRow（button）并列，不能嵌套
function BranchRow({
  refName,
  isHead,
  params,
  afterSelect,
  onAddWorktree,
  onDeleteBranch,
}: {
  refName: string;
  isHead: boolean;
  params: WorkbenchParams;
  afterSelect?: () => void;
  onAddWorktree: () => void;
  onDeleteBranch: () => void;
}) {
  // 选中态放整行容器（与 WorktreeRow 同构）：SelectableRow 传 bare 后自身不上底色
  const src: TreeSource = { type: 'ref', id: refName };
  const selected = sameSource(params.current, src) || sameSource(params.base, src);
  return (
    <div className={cn('group flex items-center hover:bg-accent', selected && 'bg-primary/15')}>
      <SelectableRow
        label={refShortName(refName)}
        source={{ type: 'ref', id: refName }}
        params={params}
        badge={isHead ? '当前' : undefined}
        afterSelect={afterSelect}
        bare
      />
      <div className="flex shrink-0 items-center opacity-0 transition-opacity group-hover:opacity-100">
        <Button
          variant="ghost"
          size="icon-sm"
          title="在此分支新建 worktree"
          aria-label={`在 ${refShortName(refName)} 新建 worktree`}
          onClick={onAddWorktree}
        >
          <Plus className="size-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="icon-sm"
          title="删除分支"
          aria-label={`删除分支 ${refShortName(refName)}`}
          onClick={onDeleteBranch}
        >
          <Trash2 className="size-3.5" />
        </Button>
      </div>
    </div>
  );
}

function WorktreeRow({
  wt,
  kind,
  rootPath,
  params,
  afterSelect,
  hidden,
  onToggle,
  onRemove,
  wsExpanded,
  onToggleWs,
}: {
  wt: WorktreeStatus;
  kind: TargetKind;
  rootPath: string;
  params: WorkbenchParams;
  afterSelect?: () => void;
  hidden: boolean;
  onToggle: () => void;
  onRemove: () => void;
  wsExpanded: boolean;
  onToggleWs: () => void;
}) {
  const src: TreeSource = { type: 'worktree', id: wt.path };
  const name = wt.path.split('/').pop() || wt.path;
  // hover/选中态放整行容器（前后的开关/opener 按钮同属一行，只亮中间段会很碎）
  const selected = sameSource(params.current, src) || sameSource(params.base, src);

  // workspace 子行展开态：集合由 WorktreeSection 持有（usePersistentSet），行只读写内存
  const wsList = wt.workspaces ?? [];
  const [resetOpen, setResetOpen] = useState(false);

  return (
    <div className="w-full">
      <div
        className={cn(
          'group flex items-center transition-colors hover:bg-accent',
          selected && 'bg-primary/15',
          hidden && 'opacity-50',
        )}
      >
        {wsList.length > 0 && (
          <Button
            variant="ghost"
            size="icon-sm"
            className="shrink-0"
            title={wsExpanded ? '收起 workspace' : `展开 ${wsList.length} 个 workspace`}
            aria-label={wsExpanded ? `收起 ${name} 的 workspace` : `展开 ${name} 的 workspace`}
            aria-expanded={wsExpanded}
            onClick={onToggleWs}
          >
            <ChevronRight className={cn('size-3.5 transition-transform', wsExpanded && 'rotate-90')} />
          </Button>
        )}
        <Button
          variant="ghost"
          size="icon-sm"
          title={hidden ? '在 commit 图中展示该副本' : '在 commit 图中隐藏该副本'}
          aria-label={hidden ? `展示 ${name}` : `隐藏 ${name}`}
          onClick={onToggle}
        >
          {hidden ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
        </Button>
        <SelectableRow
          label={name}
          source={src}
          params={params}
          title={wt.path}
          afterSelect={afterSelect}
          bare
          badges={
            <>
              {wt.branch ? <Badge variant="secondary">{wt.branch}</Badge> : null}
              {wt.bare ? <Badge variant="outline">bare</Badge> : null}
              {wt.ahead > 0 ? <Badge variant="secondary">↑{wt.ahead}</Badge> : null}
              {wt.behind > 0 ? <Badge variant="secondary">↓{wt.behind}</Badge> : null}
              {wt.dirty ? (
                <Badge variant="destructive" className="px-1">
                  脏 {wt.staged + wt.unstaged + wt.untracked}
                </Badge>
              ) : null}
              {wt.detached ? <Badge variant="outline">detached</Badge> : null}
            </>
          }
        />
        <div className="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity hover:opacity-100 group-hover:opacity-100">
          <Button variant="ghost" size="icon-sm" title="删除该 worktree" aria-label={`删除 ${name}`} onClick={onRemove}>
            <Trash2 className="size-3.5" />
          </Button>
        </div>
        <WorktreeOpenActions
          path={wt.path}
          name={name}
          kind={kind}
          diffBase={kind === 'worktree' ? rootPath : undefined}
          onReset={() => setResetOpen(true)}
        />
      </div>
      {resetOpen ? (
        <WorktreeResetDialog wtPath={wt.path} branch={wt.branch || undefined} onClose={() => setResetOpen(false)} />
      ) : null}
      {wsExpanded &&
        wsList.map((w) => {
          // 子行不可选中（不是 TreeSource，纯打开入口）：目录名 + 相对路径 + 各自的打开动作
          const wsDir = wt.path.replace(/\/$/, '') + '/' + w.path;
          return (
            <div
              key={wsDir}
              className={cn(
                'flex items-center pl-8 text-xs text-muted-foreground hover:bg-accent',
                hidden && 'opacity-50',
              )}
            >
              <span className="shrink-0">{w.name}</span>
              <span className="ml-2 min-w-0 truncate font-mono text-[10px] opacity-70" title={wsDir}>
                {w.path}
              </span>
              <div className="ml-auto flex shrink-0 items-center">
                <WorktreeOpenActions path={wsDir} name={w.name} kind="workspace" />
              </div>
            </div>
          );
        })}
    </div>
  );
}

// 副本行尾的 opener 动作（与 projects 页行内动作同构）：快捷图标（intent 槽位 ×
// 默认 opener）+ 全量下拉。槽位同 projects 页 quickIntents 的目标策略
// （workbench 仅主根、git 仅仓库根副本、terminal/dir 任意），未配默认则隐藏
// （不回落）；工作台额外在 dir 后加 ide 槽位。须与 SelectableRow（button）并列
// ——HTML 不允许 button 嵌套 button

// 工作台副本行的槽位清单：quickIntents + dir 后追加 ide（projects 页不加）
const workbenchQuickIntents: QuickIntent[] = [
  ...quickIntents.filter((q) => q.intent !== 'dir'),
  { intent: 'dir', targets: 'all' },
  { intent: 'ide', targets: 'all' },
];

// onReset 仅 worktree 行传入（reset 作用于整个副本，workspace 子目录不适用）；
// diffBase 仅 worktree 行传入（主目录路径，「与主目录对比」入口，diff-dir 意图
// 默认 opener 承载——未配默认则隐藏，不回落）
function WorktreeOpenActions({
  path,
  name,
  kind,
  diffBase,
  onReset,
}: {
  path: string;
  name: string;
  kind: TargetKind;
  diffBase?: string;
  onReset?: () => void;
}) {
  const openers = useOpenerList();
  const open = useOpenerOpen();
  const diffOpen = useOpenerDiffOpen();
  const openerList = openers.data?.list ?? [];
  const defaultOpenerOf = useIntentDefaultOpener();
  const diffOp = diffBase ? defaultOpenerOf('diff-dir') : undefined;
  const onOpen = (opener: string) => open.run(opener, path, true);

  return (
    <div className="flex shrink-0 items-center gap-0.5">
      {workbenchQuickIntents
        .filter((q) => filterTargets([{ dir: path, label: name, kind }], q.targets).length > 0)
        .map((q) => {
          const op = defaultOpenerOf(q.intent);
          if (!op) return null; // 未配默认的槽位隐藏，不回落
          return (
            <Button
              key={q.intent}
              variant="ghost"
              size="icon-sm"
              title={op.title}
              aria-label={`${op.title}（${name}）`}
              disabled={open.isPending && open.variables?.opener === op.name}
              onClick={() => onOpen(op.name)}
            >
              {renderIcon(op?.icon, null)}
            </Button>
          );
        })}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label={`打开 ${name}`} />}>
          <Ellipsis className="size-3.5" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-auto min-w-56">
          <CopyPathItem path={path} />
          {onReset ? (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={onReset}>
                <RotateCcw className="mr-1 size-3" />
                重置到指定位置
              </DropdownMenuItem>
            </>
          ) : null}
          {diffBase && diffOp ? (
            <DropdownMenuItem onClick={() => diffOpen.run(diffOp.name, diffBase, path)}>
              <GitCompare className="mr-1 size-3" />
              与主目录对比
            </DropdownMenuItem>
          ) : null}
          <DropdownMenuSeparator />
          <OpenWithGroup
            openerList={openerList}
            renderItem={(op) => (
              <DropdownMenuItem key={op.name} onClick={() => onOpen(op.name)}>
                {renderIcon(op?.icon, null)}
                {op.title}
              </DropdownMenuItem>
            )}
          />
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

