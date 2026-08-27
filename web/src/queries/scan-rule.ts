// scan/clone 规则增删改（数据落 settings.json，经 Web API 写，保存即生效）。
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { apiGet, apiPost } from '@/api/client';

const RULE_KEYS = ['project', 'scan-rules'] as const;
const CLONE_KEYS = ['project', 'clone-rules'] as const;

export function useScanRules() {
  return useQuery({ queryKey: [...RULE_KEYS], queryFn: () => apiGet('/api/project/scan-rules') });
}

export function useCloneRules() {
  return useQuery({ queryKey: [...CLONE_KEYS], queryFn: () => apiGet('/api/project/clone-rules') });
}

export function useScanRuleSave() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Parameters<typeof apiPost<'/api/project/scan-rule/save'>>[1]) =>
      apiPost('/api/project/scan-rule/save', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...RULE_KEYS] }),
  });
}

export function useScanRuleDelete() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { path: string }) => apiPost('/api/project/scan-rule/delete', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...RULE_KEYS] }),
  });
}

// 拖拽排序：提交按目标顺序排列的全量名单（乐观更新在调用方，失败时 invalidate 回滚）
export function useScanRuleReorder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Parameters<typeof apiPost<'/api/project/scan-rule/reorder'>>[1]) =>
      apiPost('/api/project/scan-rule/reorder', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...RULE_KEYS] }),
    onError: () => void qc.invalidateQueries({ queryKey: [...RULE_KEYS] }),
  });
}

export function useCloneRuleSave() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Parameters<typeof apiPost<'/api/project/clone-rule/save'>>[1]) =>
      apiPost('/api/project/clone-rule/save', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...CLONE_KEYS] }),
  });
}

export function useCloneRuleDelete() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { repoHost: string; repoPrefix: string }) => apiPost('/api/project/clone-rule/delete', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...CLONE_KEYS] }),
  });
}

export function useCloneRuleReorder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Parameters<typeof apiPost<'/api/project/clone-rule/reorder'>>[1]) =>
      apiPost('/api/project/clone-rule/reorder', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...CLONE_KEYS] }),
    onError: () => void qc.invalidateQueries({ queryKey: [...CLONE_KEYS] }),
  });
}
