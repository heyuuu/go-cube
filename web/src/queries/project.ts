import { useMutation, useQuery } from '@tanstack/react-query';

import { apiGet, apiPost } from '@/api/client';
import { tryOpenUrlAction } from '@/lib/opener-action';
import { useOpenerList } from '@/queries/opener';
import { useRecordUsage } from '@/queries/usage';

// 30s 轮询：后端 gitcache 由 server 定时刷新，前端只拉快照不触发采集
export function useProjectList() {
  return useQuery({
    queryKey: ['project', 'list'],
    queryFn: () => apiGet('/api/project/list'),
    refetchInterval: 30_000,
  });
}

// 用指定 opener 打开任意文件/目录——workbench/md 等通用打开口。
// url 型动作当前浏览器新 tab 直开（同源 + 不跳出当前浏览器），其余走后端
// opener/open（role 由后端按路径类型校验）。url 直开不经过后端打开链路，
// 目录打开靠 usage/record 触发接口补记（文件打开不记，usage 口径是项目/目录打开）。
export function useOpenerOpen() {
  const openers = useOpenerList();
  const record = useRecordUsage();
  const open = useMutation({
    mutationFn: (input: { path: string; opener: string }) => apiPost('/api/opener/open', input),
  });
  const run = (opener: string, path: string, isDir: boolean, opts?: { onError?: (e: Error) => void }) => {
    const op = (openers.data?.list ?? []).find((o) => o.name === opener);
    if (op && tryOpenUrlAction(op.actions?.[isDir ? 'open-dir' : 'open-file'], [path])) {
      if (isDir) record.mutate({ project: path, opener });
      return;
    }
    open.mutate({ path, opener }, { onError: opts?.onError });
  };
  return { ...open, run };
}

// 用指定 opener 对比两个目录（diff-dir 意图场景）。url 型动作当前浏览器新 tab
// 直开，其余走后端 opener/diff-open（role 由后端按两侧路径类型校验）。
export function useOpenerDiffOpen() {
  const openers = useOpenerList();
  const open = useMutation({
    mutationFn: (input: { left: string; right: string; opener: string }) => apiPost('/api/opener/diff-open', input),
  });
  const run = (opener: string, left: string, right: string, opts?: { onError?: (e: Error) => void }) => {
    const op = (openers.data?.list ?? []).find((o) => o.name === opener);
    if (op && tryOpenUrlAction(op.actions?.['diff-dir'], [left, right])) return;
    open.mutate({ left, right, opener }, { onError: opts?.onError });
  };
  return { ...open, run };
}

// 打开已收录项目（后端记 usage；dir 为目标目录——worktree 归并为项目打开目标，1032）。
// url 型动作当前浏览器新 tab 直开——此路径不经过后端，靠 usage/record 触发接口补记
// （入参口径与后端 projectOpen 一致：project 恒主项目路径，dir 直接传目标目录），
// 其余走后端 project/open。
export function useProjectOpen() {
  const openers = useOpenerList();
  const record = useRecordUsage();
  const open = useMutation({
    mutationFn: (input: { path: string; opener: string; dir?: string }) => apiPost('/api/project/open', input),
  });
  const run = (opener: string, path: string, dir?: string, opts?: { onError?: (e: Error) => void }) => {
    const op = (openers.data?.list ?? []).find((o) => o.name === opener);
    if (op && tryOpenUrlAction(op.actions?.['open-dir'], [dir ?? path])) {
      record.mutate({ project: path, opener, dir });
      return;
    }
    open.mutate({ path, opener, dir }, { onError: opts?.onError });
  };
  return { ...open, run };
}

// 项目 workspace 声明状态（1030）：生效清单 / 显式声明 / 探测候选（编辑视图用）
export function useWorkspaceState(path: string | null) {
  return useQuery({
    queryKey: ['project', 'workspace', path],
    queryFn: () => apiGet('/api/project/workspace/get', { path: path! }),
    enabled: !!path,
  });
}

// 保存显式 workspaces 声明（写入项目内 .cube/cube.json，后端即时重采集）
export function useWorkspaceSave() {
  return useMutation({
    mutationFn: (input: { path: string; workspaces: { name: string; path: string }[] }) =>
      apiPost('/api/project/workspace/save', input),
  });
}
