// Projects 页的筛选/排序纯函数与常量：类型、匹配谓词、下拉清单。
import type { Project } from '@/api/client';

export type GitStatus = 'clean' | 'dirty' | 'ahead' | 'behind' | 'none';

// 排序键：default=原始扫描序；其余为 键 / 键-desc（点击表头循环 default → 键 → 键-desc → default）
export type SortKey = 'name' | 'group' | 'recent';
export type SortMode = 'default' | SortKey | `${SortKey}-desc`;

export const sortKeys: { value: SortKey; label: string }[] = [
  { value: 'recent', label: '最近使用' },
  { value: 'name', label: '名称' },
  { value: 'group', label: '分组' },
];

export function isSortMode(v: string | null): v is SortMode {
  if (v === 'default') return true;
  return sortKeys.some((k) => v === k.value || v === `${k.value}-desc`);
}

export const gitFilters: { value: GitStatus | 'all'; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'clean', label: 'clean' },
  { value: 'dirty', label: 'dirty' },
  { value: 'ahead', label: 'ahead' },
  { value: 'behind', label: 'behind' },
  { value: 'none', label: '未采集' },
];

// workspace 总数：主根 + 各 worktree 下的 workspace 成员合计（列表快照字段，读路径零探测）
export function countWorkspaces(p: Project): number {
  const g = p.gitInfo;
  if (!g) return 0;
  return (g.workspaces?.length ?? 0) + (g.worktrees ?? []).reduce((n, w) => n + (w.workspaces?.length ?? 0), 0);
}

export function timeOf(iso: string | null | undefined): number {
  if (!iso) return 0;
  const t = new Date(iso).getTime();
  return Number.isNaN(t) || t <= 0 ? 0 : t;
}

// 谓词式匹配（非互斥分桶）：项目可能同时 dirty + ahead，
// 筛 ahead 应包含所有 ahead > 0 的项目，而不是被 dirty 优先级吞掉。
export function matchGitFilter(p: Project, filter: GitStatus | 'all'): boolean {
  const g = p.gitInfo;
  if (filter === 'all') return true;
  if (!g) return filter === 'none';
  switch (filter) {
    case 'none':
      return false;
    case 'dirty':
      return g.dirty;
    case 'ahead':
      return g.ahead > 0;
    case 'behind':
      return g.behind > 0;
    case 'clean':
      return !g.dirty && g.ahead === 0 && g.behind === 0;
  }
}

