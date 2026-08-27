// 新建 worktree 的默认分支名生成（提案 1031 前端优化）：
// worktree-%02d 从 01 起取第一个不与现有本地分支冲突的编号。
// 纯函数放独立模块（非 .tsx）——组件文件只导出组件（fast refresh），且可直配 vitest。

import { refShortName } from './params';

export function defaultWorktreeBranchName(localRefs: string[]): string {
  const existing = new Set(localRefs.map(refShortName));
  for (let n = 1; ; n++) {
    const name = `worktree-${String(n).padStart(2, '0')}`;
    if (!existing.has(name)) return name;
  }
}
