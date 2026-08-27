import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { selectCurrent, writePathParam, type TreeSource } from '@/pages/workbench/params';
import { useWorkbenchRefs } from '@/queries/workbench';
import {
  useBranchAdd,
  useBranchDelete,
  useWorktreeAdd,
  useWorktreeRemove,
  type WorktreeStatus,
} from '@/queries/workbench';

import { defaultWorktreeBranchName } from '../branch-name';

// worktree / 分支写侧对话框（提案 1031）。风格沿用 ConfirmDialog 的轻量自绘 overlay；
// 表单比确认场景复杂（输入 + 勾选 + 错误重试），单独成文件不复用那个两按钮壳。

// --- 新增 worktree ---

export function WorktreeAddDialog({
  path,
  prefill,
  onClose,
}: {
  path: string;
  prefill: { branch?: string; commitish?: string }; // 入口带入的基点（分支行/commit 行）
  onClose: () => void;
}) {
  // branch 为 null = 未定（等 refs 加载后落默认名）；一旦用户输入或入口带入即固定。
  // 这样默认名随 refs 异步到达，不需要 effect 回填 setState
  const [branchOverride, setBranchOverride] = useState<string | null>(prefill.branch ?? null);
  const refs = useWorkbenchRefs(path);
  const defaultBranch = refs.data ? defaultWorktreeBranchName(refs.data.locals ?? []) : '';
  const branch = branchOverride ?? defaultBranch;
  const [commitish, setCommitish] = useState(prefill.commitish ?? '');
  const [targetPath, setTargetPath] = useState('');
  const add = useWorktreeAdd(path);
  const [, setSearchParams] = useSearchParams();

  const submit = () => {
    add.mutate(
      {
        branch: branch.trim() || undefined,
        commitish: commitish.trim() || undefined,
        targetPath: targetPath.trim() || undefined,
      },
      {
        onSuccess: (created) => {
          // 建完即选中新副本（切到现场视图）
          setSearchParams(
            (prev) => {
              const next = new URLSearchParams(prev);
              selectCurrent(next, { type: 'worktree', id: created.path } satisfies TreeSource);
              return next;
            },
            { replace: true },
          );
          onClose();
        },
      },
    );
  };

  return (
    <WriteDialogShell title="新建 worktree" onClose={onClose}>
      <Field label="分支名" hint="留空 = detached；不存在则以基点新建">
        <Input value={branch} onChange={(e) => setBranchOverride(e.target.value)} placeholder="worktree-01" autoFocus />
      </Field>
      <Field label="基点" hint="commit / 分支 / tag，留空 = HEAD">
        <Input value={commitish} onChange={(e) => setCommitish(e.target.value)} placeholder="HEAD" />
      </Field>
      <Field label="目标目录" hint="留空 = 仓库同级 <仓库名>.worktrees/<分支名>/">
        <Input value={targetPath} onChange={(e) => setTargetPath(e.target.value)} placeholder="/绝对路径" />
      </Field>
      {add.isError ? <ErrorLine message={add.error.message} /> : null}
      <DialogActions confirmText="创建" pending={add.isPending} onConfirm={submit} onCancel={onClose} />
    </WriteDialogShell>
  );
}

// --- 新建分支（不检出，与删除分支对应）---

export function BranchAddDialog({ path, onClose }: { path: string; onClose: () => void }) {
  const [branch, setBranch] = useState('');
  const [commitish, setCommitish] = useState('');
  const add = useBranchAdd(path);
  const [, setSearchParams] = useSearchParams();

  const submit = () => {
    add.mutate(
      { branch: branch.trim(), commitish: commitish.trim() || undefined },
      {
        onSuccess: () => {
          // 建完选中该分支（ref 源可浏览其内容；不切任何副本的 HEAD）
          setSearchParams(
            (prev) => {
              const next = new URLSearchParams(prev);
              selectCurrent(next, { type: 'ref', id: `refs/heads/${branch.trim()}` });
              return next;
            },
            { replace: true },
          );
          onClose();
        },
      },
    );
  };

  return (
    <WriteDialogShell title="新建分支" onClose={onClose}>
      <Field label="分支名" hint="不会切 HEAD，只创建引用；要「建并切过去」用新建 worktree">
        <Input value={branch} onChange={(e) => setBranch(e.target.value)} placeholder="feat/xxx" autoFocus />
      </Field>
      <Field label="基点" hint="commit / 分支 / tag，留空 = HEAD">
        <Input value={commitish} onChange={(e) => setCommitish(e.target.value)} placeholder="HEAD" />
      </Field>
      {add.isError ? <ErrorLine message={add.error.message} /> : null}
      <DialogActions confirmText="创建" pending={add.isPending} onConfirm={submit} onCancel={onClose} />
    </WriteDialogShell>
  );
}

// --- 删除 worktree ---

export function WorktreeRemoveDialog({
  path,
  wt,
  mainPath,
  onClose,
}: {
  path: string;
  wt: WorktreeStatus;
  mainPath: string; // 主仓库目录：删的是工作台当前目录时导航回去
  onClose: () => void;
}) {
  const [force, setForce] = useState(false);
  const [deleteBranch, setDeleteBranch] = useState(false); // 同时删除该副本检出的分支
  const [denied, setDenied] = useState<string[] | null>(null);
  const remove = useWorktreeRemove(path);
  const branchDelete = useBranchDelete(path);
  const [, setSearchParams] = useSearchParams();

  const submit = () => {
    remove.mutate(
      { targetPath: wt.path, force },
      {
        onSuccess: (res) => {
          if (res.denied) {
            // 非 force 预检拒绝：展示原因，用户勾 force 后一步重试
            setDenied(res.reasons ?? []);
            return;
          }
          if (deleteBranch && wt.branch) branchDelete.mutate({ branch: wt.branch, force: true });
          // 删的是工作台正在查看的目录 → 导航回主副本（1032 的「主项目 + 目标」模型）
          if (path === wt.path) {
            setSearchParams(
              (prev) => {
                const next = new URLSearchParams(prev);
                writePathParam(next, mainPath);
                return next;
              },
              { replace: true },
            );
          }
          onClose();
        },
      },
    );
  };

  const branch = wt.branch;
  return (
    <WriteDialogShell title="删除 worktree" onClose={onClose}>
      <div className="font-mono text-[11px] break-all text-muted-foreground">{wt.path}</div>
      <div className="flex flex-wrap gap-1">
        {branch ? <Badge>{branch}</Badge> : null}
        {wt.dirty ? <Badge tone="danger">脏 {wt.staged + wt.unstaged + wt.untracked}</Badge> : null}
        {wt.ahead > 0 ? <Badge tone="danger">↑{wt.ahead} 未推送</Badge> : null}
        {wt.detached ? <Badge>detached</Badge> : null}
      </div>
      {denied ? (
        <div className="rounded border border-destructive/40 bg-destructive/10 p-2 text-xs leading-relaxed">
          <div className="font-medium">存在未保存内容：</div>
          <ul className="mt-1 list-disc pl-4">
            {denied.map((r) => (
              <li key={r}>{r}</li>
            ))}
          </ul>
          <div className="mt-1 text-muted-foreground">勾选强制删除后重试，未提交/未推送内容将丢失。</div>
        </div>
      ) : null}
      <CheckLine checked={force} onCheckedChange={setForce} label="强制删除（丢弃未提交/未推送内容）" />
      {branch ? (
        <CheckLine checked={deleteBranch} onCheckedChange={setDeleteBranch} label={`同时删除分支 ${branch}`} />
      ) : null}
      {remove.isError || branchDelete.isError ? (
        <ErrorLine message={remove.error?.message ?? branchDelete.error?.message ?? ''} />
      ) : null}
      <DialogActions
        confirmText="删除"
        danger
        pending={remove.isPending || branchDelete.isPending}
        onConfirm={submit}
        onCancel={onClose}
      />
    </WriteDialogShell>
  );
}

// --- 删除分支 ---

export function BranchDeleteDialog({
  path,
  branch,
  onClose,
}: {
  path: string;
  branch: string; // 短名（展示与提交一致）
  onClose: () => void;
}) {
  const [force, setForce] = useState(false);
  const del = useBranchDelete(path);

  const submit = () => {
    del.mutate(
      { branch, force },
      {
        onSuccess: () => onClose(),
        // 被检出 / 未合并等拒绝直接展示中文错误，用户可勾 force 重试
      },
    );
  };

  return (
    <WriteDialogShell title="删除分支" onClose={onClose}>
      <div className="font-mono text-[11px] text-muted-foreground">{branch}</div>
      <CheckLine checked={force} onCheckedChange={setForce} label="强制删除（丢弃未合并提交）" />
      {del.isError ? <ErrorLine message={del.error.message} /> : null}
      <DialogActions confirmText="删除" danger pending={del.isPending} onConfirm={submit} onCancel={onClose} />
    </WriteDialogShell>
  );
}

// --- 轻量壳与零件 ---

function WriteDialogShell({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={onClose}>
      <div
        className="mx-4 flex w-full max-w-sm flex-col gap-2.5 rounded-lg border border-border bg-background p-4 shadow-lg"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="text-sm font-semibold">{title}</div>
        {children}
      </div>
    </div>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-xs font-medium">{label}</span>
      {children}
      {hint ? <span className="text-[10px] text-muted-foreground">{hint}</span> : null}
    </label>
  );
}

function CheckLine({
  checked,
  onCheckedChange,
  label,
}: {
  checked: boolean;
  onCheckedChange: (v: boolean) => void;
  label: string;
}) {
  return (
    <label className="flex items-center gap-2 text-xs">
      <Checkbox checked={checked} onCheckedChange={(v) => onCheckedChange(!!v)} />
      {label}
    </label>
  );
}

function Badge({ children, tone }: { children: React.ReactNode; tone?: 'danger' }) {
  return (
    <span
      className={cn(
        'rounded border px-1 py-0 text-[10px]',
        tone === 'danger'
          ? 'border-destructive/40 text-destructive'
          : 'border-muted-foreground/30 text-muted-foreground',
      )}
    >
      {children}
    </span>
  );
}

function ErrorLine({ message }: { message: string }) {
  return <div className="rounded border border-destructive/40 bg-destructive/10 p-2 text-xs">{message}</div>;
}

function DialogActions({
  confirmText,
  danger,
  pending,
  onConfirm,
  onCancel,
}: {
  confirmText: string;
  danger?: boolean;
  pending?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <div className="mt-1 flex justify-end gap-2">
      <Button variant="outline" size="sm" onClick={onCancel}>
        取消
      </Button>
      <Button variant={danger ? 'destructive' : 'default'} size="sm" disabled={pending} onClick={onConfirm}>
        {confirmText}
      </Button>
    </div>
  );
}
