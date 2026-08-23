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

// 旧路径相对新路径所在目录的相对路径（仓库内相对路径专用，以 / 分段）：
// xx/a.md → xx/b.md 得 ./a.md；xx/a.md → yy/a.md 得 ../xx/a.md。
// 同目录改名时若只显示旧 basename，会看不出旧文件在哪个目录，故显式带 ./ 或 ../ 前缀。
export function relativeFilePath(newPath: string, oldPath: string): string {
  const newDir = newPath.split('/').slice(0, -1);
  const oldSegs = oldPath.split('/');
  let common = 0;
  while (common < newDir.length && common < oldSegs.length - 1 && newDir[common] === oldSegs[common]) {
    common++;
  }
  const ups = newDir.length - common;
  const rest = oldSegs.slice(common).join('/');
  if (ups === 0) return './' + rest;
  return '../'.repeat(ups) + rest;
}
