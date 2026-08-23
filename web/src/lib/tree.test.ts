import { describe, expect, it } from 'vitest';

import { buildFileTree, computeDirStatuses, flattenFileTree } from './tree';

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

describe('根不参与单链折叠', () => {
  it('根下唯一子链保持为根的子节点（折叠链仍生效），不随根隐藏', () => {
    const root = buildFileTree('', ['xxx/yyy/m.md']);
    expect(root.children.map((n) => n.name)).toEqual(['xxx/yyy']);
    expect(root.children[0]!.path).toBe('xxx/yyy');
    expect(root.children[0]!.children.map((n) => n.name)).toEqual(['m.md']);
  });
});

describe('computeDirStatuses（目录状态推导）', () => {
  const st = (status: string, oldPath?: string) => ({ status, oldPath });

  it('新目录（之前无子文件）→ added；往老目录加文件 → modified', () => {
    const stats = new Map([
      ['new/a.md', st('added')],
      ['src/new.ts', st('added')],
    ]);
    // src 下有未变更的 old.ts → src 在基准版就存在
    const out = computeDirStatuses(stats, ['new/a.md', 'src/new.ts', 'src/old.ts']);
    expect(out.get('new')).toBe('added');
    expect(out.get('src')).toBe('modified');
  });

  it('目录下全删除且无幸存子文件 → deleted；仍有幸存文件 → modified', () => {
    const gone = new Map([['a/gone.ts', st('deleted')]]);
    expect(computeDirStatuses(gone, ['other.txt']).get('a')).toBe('deleted');
    expect(computeDirStatuses(gone, ['other.txt', 'a/keep.ts']).get('a')).toBe('modified');
  });

  it('子文件全部 rename 自同一旧目录且旧位置已空 → renamed', () => {
    const stats = new Map([
      ['yy/a.md', st('renamed', 'xx/a.md')],
      ['yy/b.md', st('renamed', 'xx/b.md')],
    ]);
    const out = computeDirStatuses(stats, ['yy/a.md', 'yy/b.md']);
    expect(out.get('yy')).toBe('renamed');
  });

  it('旧位置仍有文件（部分搬走）或旧父目录不唯一：新目录仍算 added', () => {
    const partial = new Map([['yy/a.md', st('renamed', 'xx/a.md')]]);
    // xx/b.md 未动 → xx 仍有子文件，yy 不算整目录搬移；但 yy 之前不存在 → 新增
    expect(computeDirStatuses(partial, ['yy/a.md', 'xx/b.md']).get('yy')).toBe('added');

    const mixed = new Map([
      ['yy/a.md', st('renamed', 'xx/a.md')],
      ['yy/b.md', st('renamed', 'zz/b.md')],
    ]);
    expect(computeDirStatuses(mixed, ['yy/a.md', 'yy/b.md']).get('yy')).toBe('added');
  });

  it('内部挪动（a/b → a/c）：a 是 modified，a/c 是 renamed', () => {
    const stats = new Map([['a/c/f.md', st('renamed', 'a/b/f.md')]]);
    const out = computeDirStatuses(stats, ['a/c/f.md']);
    expect(out.get('a')).toBe('modified');
    expect(out.get('a/c')).toBe('renamed');
  });

  it('同目录内改名（目录本身 modified）', () => {
    const stats = new Map([['x/b.md', st('renamed', 'x/a.md')]]);
    expect(computeDirStatuses(stats, ['x/b.md']).get('x')).toBe('modified');
  });

  it('嵌套聚合：深层目录删除，父目录是 modified', () => {
    const stats = new Map([['a/b/gone.ts', st('deleted')]]);
    const out = computeDirStatuses(stats, ['a/keep.ts']);
    expect(out.get('a')).toBe('modified');
    expect(out.get('a/b')).toBe('deleted');
  });
});
