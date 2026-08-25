// opener 增删改（openers 数据落 settings.json，经 Web API 写，保存即生效）。
import { useMutation, useQueryClient } from '@tanstack/react-query';

import { apiPost } from '@/api/client';

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

// 从本地 .app 提取图标：返回 base64 PNG 字符串
export function useIconExtract() {
  return useMutation({
    mutationFn: async (path: string): Promise<string> => {
      const data = await apiPost('/api/opener/extract-icon', { path });
      return (data as { value: string }).value;
    },
  });
}
