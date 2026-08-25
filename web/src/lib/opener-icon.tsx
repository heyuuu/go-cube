// opener 图标渲染：按 icon 声明（lucide 图名 / image base64）动态渲染，
// 无声明或名字无效时 fallback。lucide 配置值是 kebab-case 图名（如 folder-open），
// icons 映射键是 PascalCase，此处做一次转换。
import { icons, type LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';

import type { Opener } from '../api/client';

function kebabToPascal(name: string): string {
  return name
    .split('-')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join('');
}

export function renderOpenerIcon(op: Opener | undefined, fallback: ReactNode): ReactNode {
  if (op?.icon?.type === 'lucide') {
    // icons 映射无字符串索引签名；未知图名得 undefined 走 fallback
    const Ico = (icons as Record<string, LucideIcon | undefined>)[kebabToPascal(op.icon.value)];
    if (Ico) return <Ico className="size-3.5" />;
  }
  if (op?.icon?.type === 'image' && op.icon.value) {
    return <img src={`data:image/png;base64,${op.icon.value}`} alt="" className="size-3.5 rounded" />;
  }
  return fallback;
}
