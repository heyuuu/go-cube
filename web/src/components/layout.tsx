import { ExternalLink, Settings } from 'lucide-react';
import { NavLink, Outlet } from 'react-router';

import { cn } from '@/lib/utils';

// 导航只放当前可用页面；新增页面在此追加（终态地图见 docs/proposals/260811-前端栈迁移）
const navItems = [{ to: '/projects', label: 'Projects' }];

export function Layout() {
  return (
    <div className="flex h-dvh">
      <aside className="flex w-52 shrink-0 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground">
        <div className="px-4 pt-5 pb-4 text-base font-semibold tracking-wide">
          <span className="text-sidebar-primary">▣</span> cube
        </div>
        <nav className="flex flex-col gap-0.5 px-2">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                cn(
                  'rounded-md px-3 py-1.5 text-xs/relaxed text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
                  isActive && 'bg-sidebar-accent font-medium text-sidebar-accent-foreground',
                )
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
        {/* 底部固定区：Config 是整个工具的配置入口（非业务页面），与外部链接归在一起 */}
        <div className="mt-auto flex flex-col gap-0.5 px-2 pb-4">
          <NavLink
            to="/config"
            className={({ isActive }) =>
              cn(
                'flex items-center gap-2 rounded-md px-3 py-1.5 text-xs/relaxed text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
                isActive && 'bg-sidebar-accent font-medium text-sidebar-accent-foreground',
              )
            }
          >
            <Settings className="size-3" data-icon="inline-start" />
            Config
          </NavLink>
          <a
            href="/docs"
            target="_blank"
            rel="noreferrer"
            className="flex items-center gap-2 rounded-md px-3 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-foreground"
          >
            API Docs
            <ExternalLink className="size-3" />
          </a>
        </div>
      </aside>
      <main className="min-w-0 flex-1 overflow-y-auto">
        {/* 大屏收敛内容宽度并居中，避免表格被拉满全屏显得空旷 */}
        <div className="mx-auto max-w-7xl">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
