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
