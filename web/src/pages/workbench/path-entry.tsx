import { useState, type SubmitEvent } from 'react';

import { apiGet } from '@/api/client';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useRecentPaths } from '@/queries/usage';

// 工作台入口引导：输入本机 git 目录路径（不要求在 project scan 管理范围内）。
// 提交前先调后台校验是否 git 仓库，非 git 目录就地显示错误不跳转——
// 路径的调整入口只有这里（URL 里转码后的 path 参数对用户不可编辑）。
// 无待校验 path 时展示最近打开清单（usage 快照，点击直接进入，失效目录由校验兜底报错）。
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
  const recents = useRecentPaths(5);

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
      {!initial && (recents.data?.list?.length ?? 0) > 0 ? (
        <div className="flex flex-col gap-1 pt-2">
          <div className="text-center text-xs text-muted-foreground">最近打开</div>
          {recents.data!.list!.map((r) => (
            <button
              key={r.path}
              type="button"
              title={new Date(r.time).toLocaleString()}
              className="truncate rounded-md px-2 py-1 text-left text-xs font-mono text-muted-foreground hover:bg-accent hover:text-accent-foreground"
              onClick={() => onSubmit(r.path)}
            >
              {r.path}
            </button>
          ))}
        </div>
      ) : null}
    </form>
  );
}
