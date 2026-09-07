import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { apiGet, apiPost } from '@/api/client';

export function useRecentPaths(limit = 5) {
  return useQuery({
    queryKey: ['usage', 'recent-paths', limit],
    queryFn: () => apiGet('/api/usage/recent-paths', { limit }),
  });
}

// 补记一条使用记录（fire-and-forget，失败静默）：web 直开不经过后端打开链路的
// 场景（url 型 opener 动作、workbench 进入）靠它把 usage 信号补回后台。
export function useRecordUsage() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { project: string; opener?: string; dir?: string }) => apiPost('/api/usage/record', input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['usage'] }),
  });
}
