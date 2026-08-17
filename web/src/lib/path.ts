// prettyPath：绝对路径里的 HOME 前缀替换为 ~（对应后端 pathkit.PrettyPath）。
// 浏览器拿不到 HOME，从项目路径集合启发式推断「/Users/<user>」前缀。
export function guessHome(paths: string[]): string {
  for (const p of paths) {
    const m = p.match(/^(\/(?:Users|home)\/[^/]+)\//);
    if (m) return m[1];
  }
  return '';
}

export function prettyPath(p: string, home: string): string {
  if (!home) return p;
  if (p === home) return '~';
  if (p.startsWith(home + '/')) return '~' + p.slice(home.length);
  return p;
}
