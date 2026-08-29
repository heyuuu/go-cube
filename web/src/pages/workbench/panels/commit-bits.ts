// 提交展示的共享小件（git 树面板与内容面板提交详情区共用）。
// 独立成非组件文件，避免从组件文件导出常量/函数（Fast Refresh 惯例）。

// ref 徽标按类型分组配色（与泳道色无关）：本地分支蓝、远程灰、tag 琥珀、HEAD 紫；
// worktree 实心填充（其余仅描边）——副本徽标与引用徽标视觉分层
export const REF_BADGE_STYLE = {
  local: 'border-blue-500/40 text-blue-600 dark:text-blue-400',
  remote: 'border-muted-foreground/30 text-muted-foreground',
  tag: 'border-amber-500/40 text-amber-600 dark:text-amber-400',
  head: 'border-violet-500/40 text-violet-600 dark:text-violet-400',
  worktree: 'border-cyan-500/50 bg-cyan-500/15 text-cyan-700 dark:text-cyan-300',
} as const;

// 提交时间显示：今天 → 纯时间（HH:mm）；今天以前 → 纯日期（当年 MM-DD，跨年 YYYY-MM-DD）；
// 悬停 → 完整「日期 时间」
export function formatCommitTime(ts: number): { text: string; full: string } {
  const d = new Date(ts * 1000);
  const now = new Date();
  const sameDay =
    d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth() && d.getDate() === now.getDate();
  const hm = `${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
  const ymd = sameYear(d, now)
    ? `${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
    : `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
  const full = `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} ${hm}`;
  return { text: sameDay ? hm : ymd, full };
}

function sameYear(a: Date, b: Date): boolean {
  return a.getFullYear() === b.getFullYear();
}

function pad2(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}
