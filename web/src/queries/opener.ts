// 打开意图（openerIntents 节，1038）：intent → 默认 opener + 候选清单。
// 候选缺省 = 声明了对应 role 的全部 opener（读侧合成，openers 数组可覆盖）。
export function useOpenerIntents() {
  return useQuery({
    queryKey: ['opener', 'intents'],
    queryFn: async () =>
        ((await apiGet('/api/opener/intents')).list ?? []).map((i) => ({ ...i, openers: i.openers ?? [] })),
  });
}

// opener 清单（openers 节）：project / workbench / md 等打开入口共用。
export function useOpenerList() {
  return useQuery({ queryKey: ['opener', 'list'], queryFn: () => apiGet('/api/opener/list') });
}

export function useIntentDefaultSave() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { intent: string; opener: string }) => apiPost('/api/opener/intent-default/save', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['opener', 'intents'] }),
  });
}

export function useIntentDefaultDelete() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { intent: string }) => apiPost('/api/opener/intent-default/delete', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['opener', 'intents'] }),
  });
}

// 解析某 intent 的默认 opener（组合 intents + openers 两个查询）。
// 返回取值函数：未配默认或 opener 失效（读侧已清理，这里兜底）时 undefined——
// 调用方隐藏入口，不回落其它 intent 的默认。
export function useIntentDefaultOpener() {
  const intents = useOpenerIntents();
  const openers = useOpenerList();
  return (intent: string): Opener | undefined => {
    const def = intents.data?.find((i) => i.intent === intent)?.defaultOpener;
    if (!def) return undefined;
    return (openers.data?.list ?? []).find((o) => o.name === def);
  };
}

// opener 增删改（openers 数据落 settings.json，经 Web API 写，保存即生效）。
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import type { Opener } from '@/api/client';
import { apiGet, apiPost } from '@/api/client';

export function useOpenerSave() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Parameters<typeof apiPost<'/api/opener/save'>>[1]) => apiPost('/api/opener/save', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['opener', 'list'] }),
  });
}

export function useOpenerDelete() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { name: string }) => apiPost('/api/opener/delete', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['opener', 'list'] }),
  });
}

// 拖拽排序：提交按目标顺序排列的全量名单（乐观更新在调用方，失败时 invalidate 回滚）
export function useOpenerReorder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Parameters<typeof apiPost<'/api/opener/reorder'>>[1]) => apiPost('/api/opener/reorder', input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['opener', 'list'] }),
    onError: () => void qc.invalidateQueries({ queryKey: ['opener', 'list'] }),
  });
}
