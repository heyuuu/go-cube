// icon 辅助请求：图标提取（IconField「提取」入口）。
// source 三形态由后端分发：本地图片文件 / .app 目录（icns）/ http(s) URL（favicon.ico 等）。
import { useMutation } from '@tanstack/react-query';

import { apiPost } from '@/api/client';

// 提取图标（统一 64px PNG）：返回 base64 字符串
export function useIconExtract() {
  return useMutation({
    mutationFn: async (source: string): Promise<string> => (await apiPost('/api/icon/extract', { source })).value,
  });
}
