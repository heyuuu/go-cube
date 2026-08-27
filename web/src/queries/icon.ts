// icon 辅助请求（.app 图标提取）。端点暂挂 opener 路由下，语义通用，后续可迁中性路由。
import { useMutation } from '@tanstack/react-query';

import { apiPost } from '@/api/client';

// 从本地 .app 提取图标：返回 base64 PNG 字符串
export function useIconExtract() {
  return useMutation({
    mutationFn: async (path: string): Promise<string> => {
      const data = await apiPost('/api/opener/extract-icon', { path });
      return (data as { value: string }).value;
    },
  });
}
