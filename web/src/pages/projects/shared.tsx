// Projects 页共用常量：tag 徽标配色 + 行内快捷打开配置。
// 独立成文件（不放 actions.tsx）是为满足 react/only-export-components 的 fast refresh 约束。
import { FolderOpen, GitBranch } from 'lucide-react';
import type { ReactNode } from 'react';

// tag → badge 配色；未收录的 tag 落到 outline
export const tagVariants: Record<string, 'default' | 'secondary' | 'outline'> = {
  worktree: 'secondary',
  godot: 'default',
};

// 行内固定快捷打开（opener 名对应 /api/opener/list）；调整入口在此
export const quickOpens: { opener: string; title: string; icon: ReactNode }[] = [
  { opener: 'finder', title: '打开所在目录', icon: <FolderOpen className="size-3.5" /> },
  { opener: 'stree', title: '打开 Git 信息', icon: <GitBranch className="size-3.5" /> },
];
