import { useMutation, useQuery } from '@tanstack/react-query';

import { apiGet, apiPost } from '@/api/client';

// 30s 轮询：后端 gitcache 由 server 定时刷新，前端只拉快照不触发采集
export function useProjectList() {
  return useQuery({
    queryKey: ['project', 'list'],
    queryFn: () => apiGet('/api/project/list'),
    refetchInterval: 30_000,
  });
}

export function useOpenerList() {
  return useQuery({ queryKey: ['opener', 'list'], queryFn: () => apiGet('/api/opener/list') });
}

// 用指定 opener 打开任意文件/目录（role 由后端按路径类型校验）——workbench/md 等通用打开口
export function useOpenerOpen() {
  return useMutation({
    mutationFn: (input: { path: string; opener: string }) => apiPost('/api/opener/open', input),
  });
}

// 打开已收录项目（后端记 usage；dir 为目标目录——worktree 归并为项目打开目标，1032）
export function useProjectOpen() {
  return useMutation({
    mutationFn: (input: { path: string; opener: string; dir?: string }) => apiPost('/api/project/open', input),
  });
}

// 项目 workspace 声明状态（1030）：生效清单 / 显式声明 / 探测候选（编辑视图用）
export function useWorkspaceState(path: string | null) {
  return useQuery({
    queryKey: ['project', 'workspace', path],
    queryFn: () => apiGet('/api/project/workspace/get', { path: path! }),
    enabled: !!path,
  });
}

// 保存显式 workspaces 声明（写入项目内 .cube/cube.json，后端即时重采集）
export function useWorkspaceSave() {
  return useMutation({
    mutationFn: (input: { path: string; workspaces: { name: string; path: string }[] }) =>
      apiPost('/api/project/workspace/save', input),
  });
}
