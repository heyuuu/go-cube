import { describe, expect, it } from 'vitest';

import { buildFileTree, flattenFileTree } from './tree';

describe('buildFileTree 绝对根（md 页语义）', () => {
  it('组树 + 目录前置排序 + 单链折叠', () => {
    const root = buildFileTree('/docs', ['/docs/a.md', '/docs/d/b.md', '/docs/x/y/c.md']);
    // 目录在前：d、x/y（折叠链 name 合并）在 a.md 之前
    expect(root.children.map((n) => n.name)).toEqual(['d', 'x/y', 'a.md']);
    expect(root.children[0]!.path).toBe('/docs/d');
    // 折叠链：path 取最深层目录
    expect(root.children[1]!.path).toBe('/docs/x/y');
    expect(root.children[1]!.children.map((n) => n.name)).toEqual(['c.md']);
  });
});

describe('buildFileTree 空根（workbench 相对路径）', () => {
  it('根 path 为空串，子路径无前导斜杠', () => {
    const root = buildFileTree('', ['a.txt', 'b/c.txt']);
    expect(root.path).toBe('');
    expect(root.children.map((n) => n.name)).toEqual(['b', 'a.txt']);
    expect(root.children[0]!.path).toBe('b');
    expect(root.children[0]!.children[0]!.path).toBe('b/c.txt');
  });
});

describe('flattenFileTree', () => {
  const root = buildFileTree('', ['a.txt', 'b/c.txt', 'b/d/e.txt']);

  it('未展开只见根与顶层；展开 b 后见其子（含折叠链 b/d）', () => {
    const collapsed = flattenFileTree(root, () => false);
    expect(collapsed.map((r) => r.node.path)).toEqual(['', 'b', 'a.txt']);

    const opened = flattenFileTree(root, (p) => p === 'b');
    expect(opened.map((r) => r.node.path)).toEqual(['', 'b', 'b/d', 'b/c.txt', 'a.txt']);
    // 展开折叠链 b/d（path 为最深层）
    const deep = flattenFileTree(root, (p) => p === 'b' || p === 'b/d');
    expect(deep.map((r) => r.node.path)).toEqual(['', 'b', 'b/d', 'b/d/e.txt', 'b/c.txt', 'a.txt']);
  });

  it('根行恒展开且标记 isRoot', () => {
    const rows = flattenFileTree(root, () => false);
    expect(rows[0]!.isRoot).toBe(true);
    expect(rows[0]!.expanded).toBe(true);
  });
});
