// front-matter（文件头 --- 包围的 YAML 块）轻量解析。渲染剥离交给 remark-frontmatter
// 插件，这里只负责把常见扁平字段解析成键值对供页面展示元信息。不做完整 YAML 解析：
// 嵌套映射等复杂结构宁可跳过也不错显（title/date/tags 等扁平字段覆盖绝大多数文档）。

export interface FrontMatterEntry {
  key: string;
  value: string;
}

// 围栏行：--- 开 / --- 或 ... 关（与 remark-frontmatter 的 yaml preset 一致）
const FENCE_RE = /^(---|\.\.\.)\s*$/;

type Pending = { key: string; buf: string[]; sep: string; mode: 'list' | 'scalar' };

function unquote(v: string): string {
  const m = /^(['"])([\s\S]*)\1$/.exec(v);
  return m ? m[2] : v;
}

// 内联数组 [a, b] 拍平成逗号串；非数组返回 null（与「空数组」区分）
function inlineArray(v: string): string | null {
  const m = /^\[(.*)\]$/.exec(v);
  if (!m) return null;
  return m[1]
    .split(',')
    .map((s) => unquote(s.trim()))
    .filter(Boolean)
    .join(', ');
}

function parseYamlish(lines: string[]): FrontMatterEntry[] {
  const out: FrontMatterEntry[] = [];
  // 挂起条目：key 后 value 为空（等块列表）或为 |/（等多行标量），收集后续缩进行
  let pending: Pending | null = null;
  const flush = () => {
    if (!pending || pending.buf.length === 0) return;
    out.push({ key: pending.key, value: pending.buf.join(pending.sep) });
    pending = null;
  };

  for (const line of lines) {
    const t = line.trim();
    if (!t || t.startsWith('#')) continue;

    const kv = /^([^\s:]+):\s*(.*)$/.exec(t);
    if (kv && !/^\s/.test(line)) {
      flush();
      const raw = kv[2].trim();
      const inline = inlineArray(raw);
      if (inline !== null) {
        out.push({ key: kv[1], value: inline });
      } else if (raw === '') {
        pending = { key: kv[1], buf: [], sep: ', ', mode: 'list' };
      } else if (/^[|>][+-]?$/.test(raw)) {
        pending = { key: kv[1], buf: [], sep: raw[0] === '|' ? '\n' : ' ', mode: 'scalar' };
      } else {
        out.push({ key: kv[1], value: unquote(raw) });
      }
      continue;
    }

    if (!pending) continue; // 无归属的缩进行（嵌套 map 子字段等）跳过
    const item = /^-\s*(.*)$/.exec(t);
    if (item) {
      if (item[1]) pending.buf.push(unquote(item[1]));
    } else if (pending.mode === 'scalar') {
      pending.buf.push(t);
    }
    // list 模式下非列表项的缩进行（嵌套结构）忽略
  }
  flush();
  return out;
}

// parseFrontmatter 提取文件头 front-matter 并解析为可展示键值对；无 front-matter 或
// 无可识别字段时返回空数组
export function parseFrontmatter(content: string): FrontMatterEntry[] {
  const lines = content.split(/\r?\n/);
  if (!FENCE_RE.test(lines[0])) return [];
  const end = lines.findIndex((l, i) => i > 0 && FENCE_RE.test(l));
  if (end === -1) return [];
  return parseYamlish(lines.slice(1, end));
}
