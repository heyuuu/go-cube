import { useInfiniteQuery, useQuery } from '@tanstack/react-query';

import { apiGet, apiPut } from '@/api/client';
import type { components } from '@/api/schema';
import type { TreeSource } from '@/pages/workbench/params';

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

export type GraphCommit = components['schemas']['GraphCommit'];
export type GraphWire = components['schemas']['GraphWire'];

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

export type TreeEntryDTO = components['schemas']['TreeEntry'];
export type FileResult = components['schemas']['FileResult'];

// 源参数统一从 TreeSource 派生（面板铁律：query key 自包含、从 URL 参数派生）
function sourceQuery(src: TreeSource) {
  return { sourceType: src.type, sourceId: src.id };
}

export function useWorkbenchTree(path: string, src: TreeSource, dir: string) {
  return useQuery({
    queryKey: ['workbench', 'tree', path, src.type, src.id, dir],
    queryFn: () => apiGet('/api/workbench/tree', { path, ...sourceQuery(src), dir }),
    enabled: path !== '' && !!src,
  });
}

export function useWorkbenchFile(path: string, src: TreeSource, file: string) {
  return useQuery({
    queryKey: ['workbench', 'file', path, src.type, src.id, file],
    queryFn: () => apiGet('/api/workbench/file', { path, ...sourceQuery(src), file }),
    enabled: path !== '' && !!src && file !== '',
  });
}

export function saveWorkbenchFile(path: string, src: TreeSource, file: string, content: string) {
  return apiPut('/api/workbench/file', { path, ...sourceQuery(src), file }, { content });
}

export type DiffEntry = components['schemas']['DiffEntry'];
export type DiffTreesResult = components['schemas']['DiffTreesResult'];
export type FileDiffResult = components['schemas']['FileDiffResult'];
export type Hunk = components['schemas']['Hunk'];

export function useWorkbenchDiff(
  path: string,
  left: TreeSource,
  right: TreeSource,
  filters: { showIgnored: boolean; statusFilter: string; pathPrefix: string },
) {
  return useQuery({
    queryKey: ['workbench', 'diff', path, left.type, left.id, right.type, right.id, filters],
    queryFn: () =>
      apiGet('/api/workbench/diff', {
        path,
        leftType: left.type,
        leftId: left.id,
        rightType: right.type,
        rightId: right.id,
        showIgnored: filters.showIgnored,
        statusFilter: filters.statusFilter || undefined,
        pathPrefix: filters.pathPrefix || undefined,
      }),
    enabled: path !== '' && left.id !== '' && right.id !== '',
    staleTime: 0, // 对比结果实时算
  });
}

export function useWorkbenchFileDiff(path: string, left: TreeSource, right: TreeSource, file: string) {
  return useQuery({
    queryKey: ['workbench', 'fileDiff', path, left.type, left.id, right.type, right.id, file],
    queryFn: () =>
      apiGet('/api/workbench/file-diff', {
        path,
        leftType: left.type,
        leftId: left.id,
        rightType: right.type,
        rightId: right.id,
        file,
      }),
    enabled: path !== '' && file !== '' && left.id !== '' && right.id !== '',
    staleTime: 0,
  });
}

// 差异模式：源相对上一版本的变更文件（commit/ref vs 父提交；worktree vs HEAD）
export function useWorkbenchChanges(path: string, src: TreeSource, enabled: boolean) {
  return useQuery({
    queryKey: ['workbench', 'changes', path, src.type, src.id],
    queryFn: () => apiGet('/api/workbench/changes', { path, ...sourceQuery(src) }),
    enabled: enabled && path !== '' && !!src,
    staleTime: 0,
  });
}
