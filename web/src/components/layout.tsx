import { ExternalLink } from 'lucide-react';
import { NavLink, Outlet } from 'react-router';

import { cn } from '@/lib/utils';

// 导航只放当前可用页面；新增页面在此追加（终态地图见 docs/proposals/260811-前端栈迁移）
const navItems = [
  { to: '/projects', label: 'Projects' },
  { to: '/config', label: 'Config' },
];

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
        <div className="mt-auto px-4 pb-4">
          <a
            href="/docs"
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
          >
            API Docs
            <ExternalLink className="size-3" />
          </a>
        </div>
      </aside>
      <main className="min-w-0 flex-1 overflow-y-auto">
        <Outlet />
      </main>
    </div>
  );
}
