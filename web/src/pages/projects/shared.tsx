// Projects 页共用常量：tag 徽标配色 + 行内快捷打开配置。
// 独立成文件（不放 actions.tsx）是为满足 react/only-export-components 的 fast refresh 约束。

// tag → badge 配色；未收录的 tag 落到 outline
export const tagVariants: Record<string, 'default' | 'secondary' | 'outline'> = {
  worktree: 'secondary',
  godot: 'default',
};

// 行内固定快捷打开（opener 名对应 /api/opener/list）；调整入口在此。
// title/icon 均取 opener 自身声明（后端保证恒有值），无需前端兜底。
// cube-workbench 是「在工作台打开」的 opener 形态（exec cmd `cube ui workbench`），
// 未配置时快捷位自动隐藏。
export const quickOpens: string[] = ['finder', 'stree', 'cube-workbench'];
