import { useInfiniteQuery, useQuery } from '@tanstack/react-query';

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

export type CommitEntry = components['schemas']['CommitEntry'];

// commit 图分页：useInfiniteQuery，cursor 为 skip 偏移；翻页边界按 sha 去重兜底
export function useWorkbenchCommits(path: string) {
  return useInfiniteQuery({
    queryKey: ['workbench', 'commits', path],
    initialPageParam: 0,
    queryFn: ({ pageParam }) => apiGet('/api/workbench/commits', { path, limit: 50, cursor: pageParam }),
    getNextPageParam: (last) => (last.hasMore ? last.nextCursor : undefined),
    staleTime: 30_000,
    enabled: path !== '',
  });
}

export type RepoStatus = components['schemas']['RepoStatus'];

// 工作副本状态：实时性要求高，不走 gitcache、后端直读；staleTime 短 + 窗口聚焦重取
export function useWorkbenchStatus(path: string, dir: string) {
  return useQuery({
    queryKey: ['workbench', 'status', path, dir],
    queryFn: () => apiGet('/api/workbench/status', { path, dir }),
    enabled: path !== '' && dir !== '',
    staleTime: 15_000,
    refetchOnWindowFocus: true,
  });
}
