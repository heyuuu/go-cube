import { describe, expect, it } from 'vitest';

import { relativeFilePath } from './path';

describe('relativeFilePath（rename 旧路径相对新路径所在目录）', () => {
  it.each([
    // 同目录改名：./ 前缀显式标出旧文件就在当前目录
    ['xx/b.md', 'xx/a.md', './a.md'],
    // 只改目录不改名：../ 前缀能看出旧文件原来的位置
    ['yy/a.md', 'xx/a.md', '../xx/a.md'],
    // 深层目录迁移：跨多级用多个 ../
    ['a/b/c/new.md', 'a/x/old.md', '../../x/old.md'],
    // 根下改名 / 移到根下
    ['b.md', 'a.md', './a.md'],
    ['a.md', 'xx/a.md', './xx/a.md'],
    // 部分前缀相同
    ['src/web/new.ts', 'src/cmd/old.ts', '../cmd/old.ts'],
  ])('%s ← %s 得 %s', (newPath, oldPath, expected) => {
    expect(relativeFilePath(newPath, oldPath)).toBe(expected);
  });
});
