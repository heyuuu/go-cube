import { BookOpen, Box, FolderKanban, Moon, PanelsTopLeft, Settings, Sun } from 'lucide-react';
import { useEffect } from 'react';
import { NavLink, Outlet, useLocation } from 'react-router';

import { toggleTheme, useTheme } from '@/hooks/use-theme';
import { cn } from '@/lib/utils';

import { HintTip } from './ui/tooltip';

// 全局图标栏（提案 1023）：所有业务页面（含工作台）共用的 48px 细栏杆，
// 保证从任何页面一键回 Projects。图标靠 HintTip 出 hover 标签。
// 铺满型页面（工作台多面板）不吃 max-w 收敛——由 fullBleedPrefixes 按路由前缀区分。
const navItems = [
  { to: '/projects', label: 'Projects', icon: FolderKanban },
  { to: '/workbench', label: '工作台', icon: PanelsTopLeft },
];
const fullBleedPrefixes = ['/workbench'];

const railItemClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    'flex size-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
    isActive && 'bg-sidebar-accent text-sidebar-accent-foreground',
  );

// settings 是全宽编辑界面，统一新 tab 打开（提案 1025）：业务 tab 原封不动，改完关 tab 即回
function openSettings() {
  window.open('/settings', '_blank');
}

export function Layout() {
  const theme = useTheme();
  const { pathname } = useLocation();
  const fullBleed = fullBleedPrefixes.some((p) => pathname.startsWith(p));

  // ⌘,（Ctrl+,）全局快捷键打开 settings，与 rail 图标行为一致
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === ',' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        openSettings();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  return (
    <div className="flex h-dvh">
      <aside className="flex w-12 shrink-0 flex-col items-center border-r border-sidebar-border bg-sidebar text-sidebar-foreground">
        <HintTip label="cube">
          <NavLink to="/projects" className="flex size-9 items-center justify-center" aria-label="cube 首页">
            <Box className="size-4 text-sidebar-primary" />
          </NavLink>
        </HintTip>
        <nav className="mt-2 flex flex-col gap-1 px-1.5">
          {navItems.map((item) => (
            <HintTip key={item.to} label={item.label}>
              <NavLink to={item.to} className={railItemClass} aria-label={item.label}>
                <item.icon className="size-4" />
              </NavLink>
            </HintTip>
          ))}
        </nav>
        {/* 底部固定区：Settings 是整个工具的配置入口（非业务页面），与外部链接、主题切换归在一起 */}
        <div className="mt-auto flex flex-col gap-1 px-1.5 pb-3">
          <HintTip label="设置 (⌘,)">
            <button
              type="button"
              onClick={openSettings}
              className={railItemClass({ isActive: false })}
              aria-label="设置"
            >
              <Settings className="size-4" />
            </button>
          </HintTip>
          <HintTip label="API Docs">
            <a
              href="/docs"
              target="_blank"
              rel="noreferrer"
              className="flex size-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
              aria-label="API Docs"
            >
              <BookOpen className="size-4" />
            </a>
          </HintTip>
          <HintTip label="切换主题 (⌘D)">
            <button
              type="button"
              onClick={toggleTheme}
              className={railItemClass({ isActive: false })}
              aria-label="切换主题"
            >
              {theme === 'dark' ? <Sun className="size-4" /> : <Moon className="size-4" />}
            </button>
          </HintTip>
        </div>
      </aside>
      {fullBleed ? (
        // 铺满型：页面自管滚动与内部布局（工作台多面板），main 不做收敛与滚动
        <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
          <Outlet />
        </main>
      ) : (
        <main className="min-w-0 flex-1 overflow-y-auto">
          {/* 大屏收敛内容宽度并居中，避免表格被拉满全屏显得空旷 */}
          <div className="mx-auto max-w-7xl">
            <Outlet />
          </div>
        </main>
      )}
    </div>
  );
}
