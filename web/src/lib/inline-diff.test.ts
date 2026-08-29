import { describe, expect, it } from 'vitest';

import { splitInlineDiff } from './inline-diff';

describe('splitInlineDiff', () => {
  it('相同文本全为公共片段', () => {
    const r = splitInlineDiff('abc', 'abc');
    expect(r.left).toEqual([{ text: 'abc', changed: false }]);
    expect(r.right).toEqual([{ text: 'abc', changed: false }]);
  });

  it('单字符修改只高亮变化字符', () => {
    const r = splitInlineDiff('项目根目录', '项目主目录');
    expect(r.left).toEqual([
      { text: '项目', changed: false },
      { text: '根', changed: true },
      { text: '目录', changed: false },
    ]);
    expect(r.right).toEqual([
      { text: '项目', changed: false },
      { text: '主', changed: true },
      { text: '目录', changed: false },
    ]);
  });

  it('纯新增/删除侧各自整侧高亮', () => {
    const r = splitInlineDiff('abc', 'abcdef');
    expect(r.left).toEqual([{ text: 'abc', changed: false }]);
    expect(r.right).toEqual([
      { text: 'abc', changed: false },
      { text: 'def', changed: true },
    ]);
  });

  it('空串参与配对时整行高亮', () => {
    const r = splitInlineDiff('', 'new');
    expect(r.left).toEqual([]);
    expect(r.right).toEqual([{ text: 'new', changed: true }]);
  });

  it('完全不同的文本两侧各自整行高亮', () => {
    const r = splitInlineDiff('xxx', 'yyy');
    expect(r.left).toEqual([{ text: 'xxx', changed: true }]);
    expect(r.right).toEqual([{ text: 'yyy', changed: true }]);
  });

  it('片段按相邻同类合并', () => {
    const r = splitInlineDiff('aXbXc', 'aYbYc');
    expect(r.left).toEqual([
      { text: 'a', changed: false },
      { text: 'X', changed: true },
      { text: 'b', changed: false },
      { text: 'X', changed: true },
      { text: 'c', changed: false },
    ]);
  });

  it('超长行退化为整行 changed', () => {
    const long = 'x'.repeat(1001);
    const r = splitInlineDiff(long, long + 'y');
    expect(r.left).toEqual([{ text: long, changed: true }]);
    expect(r.right).toEqual([{ text: long + 'y', changed: true }]);
  });
});
