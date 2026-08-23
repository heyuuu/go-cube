import { useCallback, useEffect, useState } from 'react';

// git 树面板的工作副本显隐开关持久化：关闭的副本不在 commit 图注入（虚拟节点/装饰）。
// 纯视图偏好，存 localStorage、以工作台 path 为 key 隔离（不同仓库互不影响），不进 URL。
// 与 workbench-layout.ts 同款读写方式：首渲染直接读 localStorage（无 SSR），变更即写回。

function storageKey(path: string) {
  return `cube.workbench.hiddenWorktrees.${encodeURIComponent(path)}`;
}

function loadHidden(path: string): Set<string> {
  try {
    const raw = localStorage.getItem(storageKey(path));
    if (!raw) return new Set();
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return new Set();
    return new Set(parsed.filter((p): p is string => typeof p === 'string'));
  } catch {
    return new Set();
  }
}

export function useWorktreeVisibility(path: string) {
  // path 变化（更换目录）时重读对应 key
  const [hidden, setHidden] = useState<Set<string>>(() => loadHidden(path));
  useEffect(() => {
    setHidden(loadHidden(path));
  }, [path]);

  useEffect(() => {
    localStorage.setItem(storageKey(path), JSON.stringify([...hidden]));
  }, [path, hidden]);

  const toggleWorktree = useCallback((wtPath: string) => {
    setHidden((prev) => {
      const next = new Set(prev);
      if (prev.has(wtPath)) next.delete(wtPath);
      else next.add(wtPath);
      return next;
    });
  }, []);

  return { hiddenWorktrees: hidden, toggleWorktree };
}
