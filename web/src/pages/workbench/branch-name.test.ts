import { describe, expect, it } from 'vitest';

import { defaultWorktreeBranchName } from './branch-name';

describe('defaultWorktreeBranchName', () => {
  it('空仓库从 01 起', () => {
    expect(defaultWorktreeBranchName([])).toBe('worktree-01');
  });

  it('取第一个空闲编号（规范全名入参）', () => {
    const refs = ['refs/heads/master', 'refs/heads/worktree-01', 'refs/heads/worktree-02'];
    expect(defaultWorktreeBranchName(refs)).toBe('worktree-03');
  });

  it('中间有空号则回填', () => {
    const refs = ['refs/heads/worktree-01', 'refs/heads/worktree-03'];
    expect(defaultWorktreeBranchName(refs)).toBe('worktree-02');
  });

  it('忽略不匹配命名规则的分支', () => {
    const refs = ['refs/heads/develop', 'refs/heads/worktree-x', 'refs/heads/worktree'];
    expect(defaultWorktreeBranchName(refs)).toBe('worktree-01');
  });

  it('两位以上自然进位', () => {
    const refs = Array.from({ length: 11 }, (_, i) => `refs/heads/worktree-${String(i + 1).padStart(2, '0')}`);
    expect(defaultWorktreeBranchName(refs)).toBe('worktree-12');
  });
});
