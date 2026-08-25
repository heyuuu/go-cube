// opener 打开动作的前端分发：web 形态直接路由跳转（不发 open 请求），
// exec 形态回调上层走 /api/opener/open。供 projects / workbench 共用。
import type { Opener } from '../api/client';

export function openWithOpener(
  openerList: Opener[],
  name: string,
  path: string,
  onOpen: (path: string, opener: string) => void,
) {
  const op = openerList.find((o) => o.name === name);
  if (op?.type === 'web') {
    window.open(`/workbench?path=${encodeURIComponent(path)}`, '_blank');
    return;
  }
  onOpen(path, name);
}
