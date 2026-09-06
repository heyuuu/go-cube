import { describe, expect, it } from 'vitest';

import { matchForgeFilter, repoHostOf, repoHostsOf } from './forge';

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

// 筛选语义（1042 起入参为项目全部 remote host）：任一 host 命中即匹配；other = 有 remote 未命中；none = 无 remote
describe('matchForgeFilter', () => {
  const hosts = ['github.com', 'gitee.com'];

  it('all 恒真', () => {
    expect(matchForgeFilter([], 'all', hosts)).toBe(true);
    expect(matchForgeFilter(['github.com'], 'all', hosts)).toBe(true);
  });

  it('具体 forge：任一 remote host 精确匹配（多 remote 项目双源均命中）', () => {
    expect(matchForgeFilter(['github.com', 'gitee.com'], 'github.com', hosts)).toBe(true);
    expect(matchForgeFilter(['github.com', 'gitee.com'], 'gitee.com', hosts)).toBe(true);
    expect(matchForgeFilter(['gitlab.com'], 'github.com', hosts)).toBe(false);
    expect(matchForgeFilter([], 'github.com', hosts)).toBe(false);
  });

  it('other：有 remote 但没有任何 host 命中已配置 forge', () => {
    expect(matchForgeFilter(['gitlab.com'], 'other', hosts)).toBe(true);
    // 有 remote 但 URL 无法解析产生不了 host，同样算未命中
    expect(matchForgeFilter([], 'other', hosts)).toBe(false);
    expect(matchForgeFilter(['github.com', 'gitlab.com'], 'other', hosts)).toBe(false);
  });

  it('none：无 remote（空 host 列表）', () => {
    expect(matchForgeFilter([], 'none', hosts)).toBe(true);
    expect(matchForgeFilter(['github.com'], 'none', hosts)).toBe(false);
  });
});

// 项目的全部 forge host：repoUrl（origin）在前，remotes 按序去重
describe('repoHostsOf', () => {
  it('多 remote 双源项目产出多 host', () => {
    expect(
      repoHostsOf({
        repoUrl: 'git@github.com:heyuuu/go-lombok.git',
        remotes: [
          { name: 'origin', url: 'git@github.com:heyuuu/go-lombok.git' },
          { name: 'gitee', url: 'git@gitee.com:heyuuu/go-lombok.git' },
        ],
      }),
    ).toEqual(['github.com', 'gitee.com']);
  });

  it('同 host 去重；remotes 与 repoUrl host 不同则都保留', () => {
    expect(
      repoHostsOf({
        repoUrl: 'https://github.com/a/b.git',
        remotes: [
          { name: 'origin', url: 'https://github.com/a/b.git' },
          { name: 'mirror', url: 'https://gitea.example.com:3000/a/b.git' },
        ],
      }),
    ).toEqual(['github.com', 'gitea.example.com:3000']);
  });

  it('空/未采集返回空数组', () => {
    expect(repoHostsOf(undefined)).toEqual([]);
    expect(repoHostsOf({})).toEqual([]);
    expect(repoHostsOf({ repoUrl: '', remotes: [] })).toEqual([]);
  });
});
