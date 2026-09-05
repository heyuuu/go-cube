import { describe, expect, it } from 'vitest';

import { repoHostOf } from './forge';

// 语义与后端 forge.RepoHost 对齐（见 server/forge/types.go 的 TestRepoHost）
describe('repoHostOf', () => {
  it('解析 ssh 形态 git@host:path', () => {
    expect(repoHostOf('git@github.com:heyuuu/cube.git')).toBe('github.com');
  });

  it('解析 https/http 形态（含端口）', () => {
    expect(repoHostOf('https://github.com/heyuuu/cube.git')).toBe('github.com');
    expect(repoHostOf('http://gitea.example.com:3000/heyuuu/cube.git')).toBe('gitea.example.com:3000');
  });

  it('解析 ssh url 形态（带用户与端口）', () => {
    expect(repoHostOf('ssh://git@gitea.example.com:2222/heyuuu/cube.git')).toBe('gitea.example.com:2222');
  });

  it('归一化大小写与尾部点号', () => {
    expect(repoHostOf('git@GitHub.COM:heyuuu/cube.git')).toBe('github.com');
    expect(repoHostOf('https://GitHub.COM./heyuuu/cube')).toBe('github.com');
  });

  it('空值/无法解析返回空串', () => {
    expect(repoHostOf(undefined)).toBe('');
    expect(repoHostOf(null)).toBe('');
    expect(repoHostOf('')).toBe('');
    expect(repoHostOf('not a url at all')).toBe('');
  });
});
