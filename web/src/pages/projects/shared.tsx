// Projects 页共用常量与纯逻辑：tag 徽标配色 + 打开目标推导 + 行内快捷打开配置。
// 独立成文件（不放 actions.tsx）是为满足 react/only-export-components 的 fast refresh 约束。

import type { Project } from '@/api/client';

// tag → badge 配色；未收录的 tag 落到 outline（worktree tag 已随 1032 归并移除）
export const tagVariants: Record<string, 'default' | 'secondary' | 'outline'> = {
  godot: 'default',
};

export type TargetKind = 'root' | 'worktree' | 'workspace';

export interface ProjectTarget {
  dir: string; // 目标目录绝对路径（根目录为空串 = 项目根）
  label: string; // 展示名（已去歧义，见 projectTargets）
  kind: TargetKind;
}

// 项目的打开目标列表（1032 根目录/worktrees + 1030 workspaces）。展示名与后端同规则：
// worktree 分支名优先（detached / 同分支冲突回退目录名），workspace 取声明/推导名。
// 排序与后端 targetEntries 一致：主根 > 主 workspaces > 各 worktree 根 > 其 workspaces。
// label 去歧义：worktree 下的 workspace 恒带归属前缀「分支 · 名字」；前缀后仍撞名
// （跨 worktree 同名、或与 worktree 根/主 workspace 撞名）的条目追加相对路径后缀。
export function projectTargets(p: Project): ProjectTarget[] {
  const g = p.gitInfo;
  const wts = g?.worktrees ?? [];
  const branchCount = new Map<string, number>();
  for (const w of wts) if (w.branch) branchCount.set(w.branch, (branchCount.get(w.branch) ?? 0) + 1);
  const joinDir = (root: string, rel: string) => root.replace(/\/$/, '') + '/' + rel;

  type Entry = ProjectTarget & { rel: string };
  const entries: Entry[] = [{ dir: '', label: '主目录', kind: 'root', rel: '' }];
  for (const w of g?.workspaces ?? []) {
    entries.push({ dir: joinDir(p.path, w.path), label: w.name, kind: 'workspace', rel: w.path });
  }
  for (const w of wts) {
    const wl = !w.branch || (branchCount.get(w.branch) ?? 0) > 1 ? (w.path.split('/').pop() ?? w.path) : w.branch;
    entries.push({ dir: w.path, label: wl, kind: 'worktree', rel: '' });
    for (const wsw of w.workspaces ?? []) {
      entries.push({ dir: joinDir(w.path, wsw.path), label: `${wl} · ${wsw.name}`, kind: 'workspace', rel: wsw.path });
    }
  }

  // 撞名的 workspace 追加相对路径；worktree 根靠分支名本身已可区分，不追加
  const labelCount = new Map<string, number>();
  for (const e of entries) labelCount.set(e.label, (labelCount.get(e.label) ?? 0) + 1);
  return entries.map(({ rel, ...e }) => ({
    ...e,
    label: e.kind === 'workspace' && (labelCount.get(e.label) ?? 0) > 1 ? `${e.label} (${rel})` : e.label,
  }));
}

// 快捷打开位的目标策略：并非所有 opener 都需要全部目标——
//   all        任意目标（目录类 opener，finder 等）
//   repo-roots 主根 + 各 worktree 根（diff 类 opener：目录对比以仓库为单位，workspace 子目录区分无意义）
//   root-only  仅主根（工作台以主仓库为基准，目标区分无意义）
export type QuickOpen = { name: string; targets: 'all' | 'repo-roots' | 'root-only' };

export const quickOpens: QuickOpen[] = [
  { name: 'cube-workbench', targets: 'root-only' },
  { name: 'stree', targets: 'repo-roots' },
  { name: 'finder', targets: 'all' },
];

// 按策略筛目标；根目录恒在（策略不排除主根）。
export function filterTargets(targets: ProjectTarget[], policy: QuickOpen['targets']): ProjectTarget[] {
  if (policy === 'all') return targets;
  if (policy === 'root-only') return targets.filter((t) => t.kind === 'root');
  return targets.filter((t) => t.kind !== 'workspace');
}
