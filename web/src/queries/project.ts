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

// 打开已收录项目（后端记 usage；后续 1030/1032 目标目录选择在此扩展）——项目页专用
export function useProjectOpen() {
  return useMutation({
    mutationFn: (input: { path: string; opener: string }) => apiPost('/api/project/open', input),
  });
}
