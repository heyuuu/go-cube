import { describe, expect, it } from 'vitest';

import { matchForgeFilter, repoHostOf } from './forge';

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

// 筛选语义：具体 forge 只看自身匹配；other = 有 remote 未命中；none = 无 remote
describe('matchForgeFilter', () => {
  const hosts = ['github.com', 'gitee.com'];

  it('all 恒真', () => {
    expect(matchForgeFilter(undefined, 'all', hosts)).toBe(true);
    expect(matchForgeFilter('git@github.com:x/y.git', 'all', hosts)).toBe(true);
  });

  it('具体 forge：host 精确匹配（不涉及其它 repo 的匹配情况）', () => {
    expect(matchForgeFilter('git@github.com:heyuuu/cube.git', 'github.com', hosts)).toBe(true);
    expect(matchForgeFilter('https://gitee.com/x/y.git', 'gitee.com', hosts)).toBe(true);
    expect(matchForgeFilter('https://gitee.com/x/y.git', 'github.com', hosts)).toBe(false);
    expect(matchForgeFilter(undefined, 'github.com', hosts)).toBe(false);
  });

  it('other：有 remote 但未命中任何已配置 forge', () => {
    expect(matchForgeFilter('https://gitlab.com/x/y.git', 'other', hosts)).toBe(true);
    // 有 remote 但 URL 无法解析同样算未命中
    expect(matchForgeFilter('weird://nope', 'other', hosts)).toBe(true);
    expect(matchForgeFilter('https://github.com/x/y.git', 'other', hosts)).toBe(false);
    expect(matchForgeFilter(undefined, 'other', hosts)).toBe(false);
  });

  it('none：无 remote（空串 / undefined）', () => {
    expect(matchForgeFilter(undefined, 'none', hosts)).toBe(true);
    expect(matchForgeFilter('', 'none', hosts)).toBe(true);
    expect(matchForgeFilter('git@github.com:x/y.git', 'none', hosts)).toBe(false);
  });
});
