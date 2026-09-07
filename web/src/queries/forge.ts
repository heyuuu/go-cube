// forge 配置与拉取（数据落 settings.json forges / forgeAccounts 节，经 Web API 写，保存即生效）。
// 1044 起 namespace 模型移除，account 是 forge 下唯一的关联配置。
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

// --- account（token 掩码语义：list 打码返回，save 提交掩码值 = 未修改） ---

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

// 拉取 account 名下全部仓库：出站 API 调用（慢操作），成功返回 count
export function useForgeAccountFetch() {
  return useMutation({
    mutationFn: (input: { forgeHost: string; username: string; force?: boolean }) =>
      apiPost('/api/forge/account/fetch', input),
  });
}

// forge 页聚合（1042/1044）：全部 account 对账行 + 拉取元信息（只读缓存，不外呼）
export function useForgeOverview() {
  return useQuery({ queryKey: [...FORGE_KEYS, 'overview'], queryFn: () => apiGet('/api/forge/overview') });
}
