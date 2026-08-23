import { useState, type SubmitEvent } from 'react';

import { apiGet } from '@/api/client';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

// 工作台入口引导：输入本机 git 目录路径（不要求在 project scan 管理范围内）。
// 提交前先调后台校验是否 git 仓库，非 git 目录就地显示错误不跳转——
// 路径的调整入口只有这里（URL 里转码后的 path 参数对用户不可编辑）。
export function PathEntry({
  initial,
  initialError,
  onSubmit,
}: {
  initial: string;
  // 带 path 进工作台但校验失败时，退回本入口页并展示的原因
  initialError?: string;
  onSubmit: (path: string) => void;
}) {
  const [value, setValue] = useState(initial);
  const [error, setError] = useState(initialError ?? '');
  const [checking, setChecking] = useState(false);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    const trimmed = value.trim();
    if (!trimmed || checking) return;
    setError('');
    setChecking(true);
    try {
      await apiGet('/api/workbench/info', { path: trimmed });
      onSubmit(trimmed);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setChecking(false);
    }
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
        <Button type="submit" size="sm" disabled={checking}>
          {checking ? '校验中…' : '进入'}
        </Button>
      </div>
      {error ? <div className="text-center text-xs text-destructive">{error}</div> : null}
      <div className="text-center text-xs text-muted-foreground">
        任意本机 git 仓库目录均可（含 worktree 目录），不要求已被 cube 扫描管理
      </div>
    </form>
  );
}
