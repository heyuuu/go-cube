import { useEffect, useState } from 'react';

type Theme = 'light' | 'dark';

const STORAGE_KEY = 'cube-theme';
const listeners = new Set<() => void>();

// 读初始主题：显式选择优先，否则跟随系统偏好
function initialTheme(): Theme {
  const saved = localStorage.getItem(STORAGE_KEY);
  if (saved === 'light' || saved === 'dark') return saved;
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

// 渲染前调用一次（main.tsx），避免首帧亮→暗闪烁
export function applyThemeEarly() {
  document.documentElement.classList.toggle('dark', initialTheme() === 'dark');
}

function applyTheme(theme: Theme) {
  localStorage.setItem(STORAGE_KEY, theme);
  document.documentElement.classList.toggle('dark', theme === 'dark');
  listeners.forEach((notify) => notify());
}

export function toggleTheme() {
  applyTheme(document.documentElement.classList.contains('dark') ? 'light' : 'dark');
}

// 全局唯一订阅：任何组件 useTheme 后都能拿到当前主题并随切换重渲染；
// 快捷键也在此挂载，保证 /workbench、/md 等不走 Layout 的路由同样生效
export function useTheme(): Theme {
  const [theme, setTheme] = useState<Theme>(() =>
    document.documentElement.classList.contains('dark') ? 'dark' : 'light',
  );

  useEffect(() => {
    const notify = () =>
      setTheme(document.documentElement.classList.contains('dark') ? 'dark' : 'light');
    listeners.add(notify);

    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'd') {
        e.preventDefault();
        toggleTheme();
      }
    };
    window.addEventListener('keydown', onKeyDown);

    return () => {
      listeners.delete(notify);
      window.removeEventListener('keydown', onKeyDown);
    };
  }, []);

  return theme;
}
