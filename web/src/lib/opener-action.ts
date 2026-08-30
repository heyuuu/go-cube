// url 型 opener 动作的前端直开：cube 页面内的打开操作对 url 动作不走后端——
// 后端会经系统 open 用默认浏览器新开窗口，既丢同源上下文，也会把从非默认浏览器
// 进来的会话甩出去。这里按动作模板在前端渲染（站内路由占位符 encodeURIComponent，
// 外部 URL 照常替换——与后端 renderURLTemplate 同规则），在当前浏览器新 tab 打开。
// 返回 false 表示非 url 动作，调用方回落后端 API。
export function tryOpenUrlAction(raw: string | undefined, paths: string[]): boolean {
  if (!raw?.startsWith('url:')) return false;
  const tmpl = raw.slice(4).trimStart();
  const external = /^https?:\/\//.test(tmpl);
  const url = tmpl.replace(/\$(\d+)/g, (all, n: string) => {
    const p = paths[Number(n)];
    return p === undefined ? all : external ? p : encodeURIComponent(p);
  });
  window.open(url, '_blank', 'noopener');
  return true;
}
