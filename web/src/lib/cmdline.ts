// opener cmd 编辑行的分词与还原：token 可含空格（如 "/Applications/Visual Studio Code.app/..."），
// 编辑文本采用 shell 风格引号约定——含空格的 token 用双引号包裹，解析时支持单/双引号。

/** shell 风格分词：空白分隔，单/双引号内的内容（含空格）为一个 token；双引号内 \" 转义；未闭合引号原样收尾。 */
export function tokenizeCmdLine(line: string): string[] {
  const tokens: string[] = [];
  let cur = '';
  let quote: '"' | "'" | null = null;
  let hasToken = false;

  const chars = [...line];
  for (let i = 0; i < chars.length; i++) {
    const ch = chars[i];
    if (ch === '\\' && (quote === '"' || quote === null) && chars[i + 1] !== undefined) {
      cur += chars[i + 1];
      hasToken = true;
      i++;
    } else if (quote) {
      if (ch === quote) quote = null;
      else cur += ch;
    } else if (ch === '"' || ch === "'") {
      quote = ch;
      hasToken = true;
    } else if (/\s/.test(ch)) {
      if (hasToken) {
        tokens.push(cur);
        cur = '';
        hasToken = false;
      }
    } else {
      cur += ch;
      hasToken = true;
    }
  }
  if (hasToken) tokens.push(cur);
  return tokens;
}

/** 还原为编辑文本：仅对含空格/引号的 token 加双引号（内部 " 转义为 \"），其余原样。 */
export function joinCmdLine(tokens: string[]): string {
  return tokens.map((t) => (/[\s"]/.test(t) ? `"${t.replaceAll('"', '\\"')}"` : t)).join(' ');
}
