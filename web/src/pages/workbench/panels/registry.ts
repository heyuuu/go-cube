import { FileCode2, GitBranch } from 'lucide-react';

// 面板注册表（提案 1015）。MVP 约束：同类型面板单实例（多实例的 URL 参数
// 命名空间设计是该提案标注的最大难点，留待需要时再做）。
// terminal 不进主区布局（固定底部抽屉），不在此注册。
// 'content' 统一 code/diff（及旧 auto）：source[+base] 视图模型，
// 单选 = 浏览+轻编辑、双选 = 与基准对比（见 content-view-panel.tsx）。

export type PanelId = 'git-tree' | 'content';

export const PANEL_REGISTRY: Record<PanelId, { label: string; icon: typeof FileCode2 }> = {
  'git-tree': { label: 'Git 树', icon: GitBranch },
  content: { label: '内容', icon: FileCode2 },
};

export const PANEL_ORDER: PanelId[] = ['content', 'git-tree'];
