import { useState, type SubmitEvent } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

// 工作台入口引导：输入本机 git 目录路径（不要求在 project scan 管理范围内）
export function PathEntry({ initial, onSubmit }: { initial: string; onSubmit: (path: string) => void }) {
  const [value, setValue] = useState(initial);
  const submit = (e: SubmitEvent) => {
    e.preventDefault();
    const trimmed = value.trim();
    if (trimmed) onSubmit(trimmed);
  };
  return (
    <form onSubmit={submit} className="mx-auto flex w-full max-w-lg flex-col gap-3 pt-24">
      <div className="text-center text-sm font-medium">打开一个 git 目录</div>
      <div className="flex gap-2">
        <Input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder="/absolute/path/to/repo"
          autoFocus
        />
        <Button type="submit" size="sm">
          进入
        </Button>
      </div>
      <div className="text-center text-xs text-muted-foreground">
        任意本机 git 仓库目录均可（含 worktree 目录），不要求已被 cube 扫描管理
      </div>
    </form>
  );
}
