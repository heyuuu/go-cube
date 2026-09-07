// 「复制绝对路径 + 打开方式」菜单块：Projects 行内动作 / workbench 副本动作 / md 树节点
// 四处同构抽取。opener 逐项渲染归调用方（各处的过滤与 pending 判定不同），此处只收敛
// 菜单壳、空态文案与「Base UI 的 GroupLabel 必须包在 Group 内」的约束。
import { Copy } from 'lucide-react';
import type { ReactNode } from 'react';

import type { Opener } from '@/api/client';
import { DropdownMenuGroup, DropdownMenuItem, DropdownMenuLabel } from '@/components/ui/dropdown-menu';

export function CopyPathItem({ path }: { path: string }) {
  return (
    <DropdownMenuItem onClick={() => void navigator.clipboard.writeText(path)}>
      <Copy className="mr-1 size-3" />
      复制绝对路径
    </DropdownMenuItem>
  );
}

export function OpenWithGroup({ openerList, renderItem }: { openerList: Opener[]; renderItem: (op: Opener) => ReactNode }) {
  return (
    <DropdownMenuGroup>
      <DropdownMenuLabel>打开方式</DropdownMenuLabel>
      {openerList.map(renderItem)}
      {openerList.length === 0 && <div className="px-2 py-1.5 text-xs text-muted-foreground">未配置 opener</div>}
    </DropdownMenuGroup>
  );
}
