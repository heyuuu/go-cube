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

// 用指定 opener 打开任意文件/目录（项目打开也走这里；role 由后端按路径类型校验）
export function useOpenerOpen() {
  return useMutation({
    mutationFn: (input: { path: string; app: string }) => apiPost('/api/opener/open', input),
  });
}
