// localStorage 偏好持久化公共 hook：md / workbench / projects 各页此前至少 7 处
// 手写「读 localStorage → 校验/clamp → 变更即写」样板，收敛至此。
// 纯浏览器 SPA 无 SSR，首渲染 lazy initializer 直接读；坏值回落 fallback。
// 写发生在 setter 内（幂等，updater 重放无害），不用 effect 以避免 key/encode
// 引用不稳引起的多余写入。
import { useCallback, useEffect, useState, type SetStateAction } from 'react';

export function useLocalPref<T>(
  key: string,
  fallback: T,
  decode: (raw: string) => T,
  encode: (v: T) => string = String,
) {
  const [value, setValue] = useState<T>(() => {
    try {
      const raw = localStorage.getItem(key);
      return raw === null ? fallback : decode(raw);
    } catch {
      return fallback;
    }
  });
  const set = useCallback(
    (v: SetStateAction<T>) => {
      setValue((prev) => {
        const next = typeof v === 'function' ? (v as (p: T) => T)(prev) : v;
        try {
          localStorage.setItem(key, encode(next));
        } catch {
          // 写失败（隐私模式/满额）不阻断状态更新——偏好丢了可接受
        }
        return next;
      });
    },
    [key, encode],
  );
  return [value, set] as const;
}

function readSet(key: string): Set<string> {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(key) ?? '[]');
    return Array.isArray(parsed) ? new Set(parsed.filter((p): p is string => typeof p === 'string')) : new Set();
  } catch {
    return new Set();
  }
}

// 字符串集合偏好（展开态 / 显隐开关等）：key 变化（如按工作台 path 隔离）时重读。
export function usePersistentSet(key: string) {
  const [set, setSet] = useState<Set<string>>(() => readSet(key));
  useEffect(() => {
    setSet(readSet(key));
  }, [key]);

  const update = useCallback(
    (fn: (prev: Set<string>) => Set<string>) => {
      setSet((prev) => {
        const next = fn(prev);
        try {
          localStorage.setItem(key, JSON.stringify([...next]));
        } catch {
          // 同上：写失败不阻断
        }
        return next;
      });
    },
    [key],
  );
  const toggle = useCallback(
    (item: string) => {
      update((prev) => {
        const next = new Set(prev);
        if (prev.has(item)) next.delete(item);
        else next.add(item);
        return next;
      });
    },
    [update],
  );
  return { set, toggle, update };
}
