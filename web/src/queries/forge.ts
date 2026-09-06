// forge 配置增删改（数据落 settings.json forges 节，经 Web API 写，保存即生效）。
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { apiGet, apiPost } from '@/api/client';

const FORGE_KEYS = ['forge'] as const;

export function useForges() {
  return useQuery({ queryKey: [...FORGE_KEYS], queryFn: () => apiGet('/api/forge/list') });
}

export function useForgeSave() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Parameters<typeof apiPost<'/api/forge/save'>>[1]) => apiPost('/api/forge/save', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...FORGE_KEYS] }),
  });
}

export function useForgeDelete() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { host: string }) => apiPost('/api/forge/delete', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...FORGE_KEYS] }),
  });
}

// 拖拽排序：提交按目标顺序排列的全量名单（乐观更新在调用方，失败时 invalidate 回滚）
export function useForgeReorder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Parameters<typeof apiPost<'/api/forge/reorder'>>[1]) => apiPost('/api/forge/reorder', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...FORGE_KEYS] }),
    onError: () => void qc.invalidateQueries({ queryKey: [...FORGE_KEYS] }),
  });
}

// --- account / namespace（提案 1041） ---

export function useForgeAccounts() {
  return useQuery({ queryKey: [...FORGE_KEYS, 'accounts'], queryFn: () => apiGet('/api/forge/account/list') });
}

export function useForgeAccountSave() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Parameters<typeof apiPost<'/api/forge/account/save'>>[1]) =>
      apiPost('/api/forge/account/save', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...FORGE_KEYS] }),
  });
}

export function useForgeAccountDelete() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { forgeHost: string; username: string }) => apiPost('/api/forge/account/delete', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...FORGE_KEYS] }),
  });
}

export function useForgeNamespaces() {
  return useQuery({ queryKey: [...FORGE_KEYS, 'namespaces'], queryFn: () => apiGet('/api/forge/namespace/list') });
}

export function useForgeNamespaceSave() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Parameters<typeof apiPost<'/api/forge/namespace/save'>>[1]) =>
      apiPost('/api/forge/namespace/save', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...FORGE_KEYS] }),
  });
}

export function useForgeNamespaceDelete() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { forgeHost: string; path: string }) => apiPost('/api/forge/namespace/delete', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...FORGE_KEYS] }),
  });
}

// 拉取：出站 API 调用（慢操作），成功/失败都让调用方感知（返回 count）
export function useForgeNamespaceFetch() {
  return useMutation({
    mutationFn: (input: { forgeHost: string; path: string; force?: boolean }) =>
      apiPost('/api/forge/namespace/fetch', input),
  });
}

// type 探测：配置表单的辅助动作（失败即提示，不阻塞手选）
export function useForgeNamespaceDetect() {
  return useMutation({
    mutationFn: (input: { forgeHost: string; path: string }) => apiPost('/api/forge/namespace/detect', input),
  });
}

// 对账：轻量即时查询（跟随拉取动作触发），不进 useQuery 缓存
export function fetchNamespaceReconcile(forgeHost: string, path: string) {
  return apiGet('/api/forge/namespace/reconcile', { forgeHost, path });
}

// forge 页聚合（1042）：全部 namespace 对账行 + 拉取元信息（只读缓存，不外呼）
export function useForgeOverview() {
  return useQuery({ queryKey: [...FORGE_KEYS, 'overview'], queryFn: () => apiGet('/api/forge/overview') });
}
