import { useQuery } from '@tanstack/react-query';

import { apiGet } from '@/api/client';
import type { components } from '@/api/schema';

export type WorkbenchInfo = components['schemas']['Info'];
export type WorkbenchRefs = components['schemas']['Refs'];
export type WorkbenchWorktree = components['schemas']['Worktree'];

// 工作台数据不走 gitcache、后端无缓存，实时性由这里的 staleTime 控制。
// info/refs 是低频基础信息，30s 内复用；status/commits 类接口（1011 引入）届时另设更短值。
export function useWorkbenchInfo(path: string) {
  return useQuery({
    queryKey: ['workbench', 'info', path],
    queryFn: () => apiGet('/api/workbench/info', { path }),
    enabled: path !== '',
    staleTime: 30_000,
  });
}

export function useWorkbenchRefs(path: string) {
  return useQuery({
    queryKey: ['workbench', 'refs', path],
    queryFn: () => apiGet('/api/workbench/refs', { path }),
    enabled: path !== '',
    staleTime: 30_000,
  });
}
