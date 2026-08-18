import { Diff, FileCode2, GitBranch, Sparkles } from 'lucide-react';

// 面板注册表（提案 1015）。MVP 约束：同类型面板单实例（多实例的 URL 参数
// 命名空间设计是该提案标注的最大难点，留待需要时再做）。
// terminal 不进主区布局（固定底部抽屉），不在此注册。
// 'auto' 是默认面板：按选中态自动呈现 code（单选）或 diff（双选）。

export type PanelId = 'git-tree' | 'code' | 'diff' | 'auto';

export const PANEL_REGISTRY: Record<PanelId, { label: string; icon: typeof FileCode2 }> = {
  'git-tree': { label: 'Git 树', icon: GitBranch },
  code: { label: '代码阅读', icon: FileCode2 },
  diff: { label: 'Diff', icon: Diff },
  auto: { label: '自动', icon: Sparkles },
};

export const PANEL_ORDER: PanelId[] = ['auto', 'git-tree', 'code', 'diff'];
