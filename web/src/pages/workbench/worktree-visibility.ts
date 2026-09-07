import { usePersistentSet } from '@/hooks/use-local-pref';

// git 树面板的工作副本显隐开关持久化：关闭的副本不在 commit 图注入（虚拟节点/装饰）。
// 纯视图偏好，存 localStorage、以工作台 path 为 key 隔离（不同仓库互不影响），不进 URL。

function storageKey(path: string) {
  return `cube.workbench.hiddenWorktrees.${encodeURIComponent(path)}`;
}

export function useWorktreeVisibility(path: string) {
  const { set, toggle } = usePersistentSet(storageKey(path));
  return { hiddenWorktrees: set, toggleWorktree: toggle };
}
