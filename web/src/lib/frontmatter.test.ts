import { describe, expect, it } from 'vitest';

import { parseFrontmatter } from './frontmatter';

describe('parseFrontmatter（front-matter 轻量解析）', () => {
  it('无 front-matter / 围栏不完整 / 正文里的 --- 不算', () => {
    expect(parseFrontmatter('# 标题\n\n正文')).toEqual([]);
    // 只有开头围栏没有闭合
    expect(parseFrontmatter('---\ntitle: a\n')).toEqual([]);
    // --- 不在文件首行，是正文分隔线不是 front-matter
    expect(parseFrontmatter('正文\n\n---\ntitle: a\n---\n')).toEqual([]);
  });

  it('扁平键值 + 引号剥离 + value 含冒号', () => {
    const fm = parseFrontmatter('---\ntitle: "hello: world"\ndate: 2026-01-02\ndraft: false\n---\n\n正文');
    expect(fm).toEqual([
      { key: 'title', value: 'hello: world' },
      { key: 'date', value: '2026-01-02' },
      { key: 'draft', value: 'false' },
    ]);
  });

  it('内联数组与块列表都拍平成逗号串', () => {
    const fm = parseFrontmatter('---\ntags: [go, "web", 工具]\ncategories:\n  - dev\n  - cube\n---\n');
    expect(fm).toEqual([
      { key: 'tags', value: 'go, web, 工具' },
      { key: 'categories', value: 'dev, cube' },
    ]);
  });

  it('多行标量 | 保留换行、> 折叠为空格', () => {
    const fm = parseFrontmatter('---\nabstract: |\n  第一行\n  第二行\ndesc: >\n  折叠\n  成一行\n---\n');
    expect(fm).toEqual([
      { key: 'abstract', value: '第一行\n第二行' },
      { key: 'desc', value: '折叠 成一行' },
    ]);
  });

  it('注释与嵌套结构不混入顶层', () => {
    const fm = parseFrontmatter('---\n# 注释行\ntitle: a\nnav:\n  home: /\n---\n');
    expect(fm).toEqual([{ key: 'title', value: 'a' }]);
  });

  it('CRLF 与 ... 闭合', () => {
    const fm = parseFrontmatter('---\r\ntitle: a\r\n...\r\n正文');
    expect(fm).toEqual([{ key: 'title', value: 'a' }]);
  });
});
