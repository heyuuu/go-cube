// 复制到剪贴板 + 1.5s「已复制」反馈：remote 地址行 / 项目抽屉复制路径等入口共用。
import { useCallback, useState } from 'react';

export function useCopied(durationMs = 1500) {
  const [copied, setCopied] = useState(false);
  const copy = useCallback(
    (text: string) => {
      void navigator.clipboard.writeText(text).then(() => {
        setCopied(true);
        window.setTimeout(() => setCopied(false), durationMs);
      });
    },
    [durationMs],
  );
  return { copied, copy };
}
