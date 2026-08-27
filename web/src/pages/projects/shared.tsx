// Projects 页共用常量与纯逻辑：tag 徽标配色 + 打开目标推导 + 行内快捷打开配置。
// 独立成文件（不放 actions.tsx）是为满足 react/only-export-components 的 fast refresh 约束。

import type { Project } from '@/api/client';

// tag → badge 配色；未收录的 tag 落到 outline（worktree tag 已随 1032 归并移除）
export const tagVariants: Record<string, 'default' | 'secondary' | 'outline'> = {
  godot: 'default',
};

// 项目的打开目标列表（1032 根目录/worktrees + 1030 workspaces）。展示名与后端同规则：
// worktree 分支名优先（detached / 同分支冲突回退目录名），workspace 取声明/推导名。
// 排序与后端 targetEntries 一致：主根 > 主 workspaces > 各 worktree 根 > 其 workspaces。
export function projectTargets(p: Project): { dir: string; label: string; kind: 'root' | 'worktree' | 'workspace' }[] {
  const g = p.gitInfo;
  const wts = g?.worktrees ?? [];
  const branchCount = new Map<string, number>();
  for (const w of wts) if (w.branch) branchCount.set(w.branch, (branchCount.get(w.branch) ?? 0) + 1);
  const joinDir = (root: string, rel: string) => root.replace(/\/$/, '') + '/' + rel;
  return [
    { dir: '', label: '根目录', kind: 'root' as const },
    ...(g?.workspaces ?? []).map((w) => ({ dir: joinDir(p.path, w.path), label: w.name, kind: 'workspace' as const })),
    ...wts.flatMap((w) => [
      {
        dir: w.path,
        label: !w.branch || (branchCount.get(w.branch) ?? 0) > 1 ? (w.path.split('/').pop() ?? w.path) : w.branch,
        kind: 'worktree' as const,
      },
      ...(w.workspaces ?? []).map((wsw) => ({
        dir: joinDir(w.path, wsw.path),
        label: wsw.name,
        kind: 'workspace' as const,
      })),
    ]),
  ];
}

// 行内固定快捷打开（opener 名对应 /api/opener/list）；调整入口在此。
// title/icon 均取 opener 自身声明（后端保证恒有值），无需前端兜底。
// cube-workbench 是「在工作台打开」的 opener 形态（exec cmd `cube ui workbench`），
// 未配置时快捷位自动隐藏。
export const quickOpens: string[] = ['finder', 'stree', 'cube-workbench'];
