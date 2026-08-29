import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { apiGet, apiPost } from '@/api/client';
import type { components } from '@/api/schema';
import { toUri, type TreeSource } from '@/pages/workbench/params';

export type WorkbenchInfo = components['schemas']['Info'];
export type WorkbenchRefs = components['schemas']['Refs'];

// 工作台数据不走 gitcache、后端无缓存，实时性由这里的 staleTime 控制。
// info/refs 是低频基础信息，30s 内复用；status/commits 类接口（1011 引入）届时另设更短值。
export function useWorkbenchInfo(path: string) {
  return useQuery({
    queryKey: ['workbench', 'info', path],
    queryFn: () => apiGet('/api/workbench/info', { path }),
    enabled: path !== '',
    staleTime: 30_000,
  });
}

export function useWorkbenchRefs(path: string) {
  return useQuery({
    queryKey: ['workbench', 'refs', path],
    queryFn: () => apiGet('/api/workbench/refs', { path }),
    enabled: path !== '',
    staleTime: 30_000,
  });
}

export type RemoteEntry = components['schemas']['RemoteEntry'];

// remote 配置列表（名字 + 抓取地址 + 网页地址），低频基础信息
export function useWorkbenchRemotes(path: string) {
  return useQuery({
    queryKey: ['workbench', 'remotes', path],
    queryFn: () => apiGet('/api/workbench/remotes', { path }),
    enabled: path !== '',
    staleTime: 30_000,
  });
}

export type CommitEntry = components['schemas']['CommitEntry'];
export type CommitRef = components['schemas']['CommitRef'];

// commit 日志分页：useInfiniteQuery，cursor 为 skip 偏移；翻页边界按 sha 去重兜底。
// 纯列表——泳道布局由前端对已持有数据计算（graph-layout.ts），接口不带几何信息
export function useWorkbenchCommits(path: string) {
  return useInfiniteQuery({
    queryKey: ['workbench', 'commits', path],
    initialPageParam: 0,
    queryFn: ({ pageParam }) => apiGet('/api/workbench/commits', { path, limit: 50, cursor: pageParam }),
    getNextPageParam: (last) => (last.hasMore ? last.nextCursor : undefined),
    staleTime: 30_000,
    refetchInterval: 30_000, // 轮询：后台可能有 AI/其他进程在改仓库，干等也要能等到变化
    enabled: path !== '',
  });
}

export type WorktreeStatus = components['schemas']['WorktreeStatus'];

export type CommitDetail = components['schemas']['CommitDetail'];

// 提交详情（含完整 message 正文，内容面板的提交详情区）：AI 时代 commit 信息长且多行，
// commit 图行内展示不全，详情区补全量展示。worktree 源无单一提交，不拉
export function useWorkbenchCommit(path: string, src: TreeSource) {
  return useQuery({
    queryKey: ['workbench', 'commit', path, toUri(src)],
    queryFn: () => apiGet('/api/workbench/commit', { path, ...sourceQuery(src) }),
    enabled: path !== '' && !!src && src.type !== 'worktree',
    staleTime: 30_000,
  });
}

// 工作副本状态快照（全部副本一次拿全）：行徽标与 commit 图虚拟节点的共同数据源。
// 实时性要求高，staleTime 短 + 轮询 + 窗口聚焦重取
export function useWorkbenchWorktrees(path: string) {
  return useQuery({
    queryKey: ['workbench', 'worktrees', path],
    queryFn: () => apiGet('/api/workbench/worktrees', { path }),
    enabled: path !== '',
    staleTime: 15_000,
    refetchInterval: 15_000,
    refetchOnWindowFocus: true,
  });
}

export type FileResult = components['schemas']['FileResult'];

// 源参数统一从 TreeSource 派生（面板铁律：query key 自包含、从 URL 参数派生），
// 传输形态为 "type://id"（后端 ParseTreeSource 的文法）
function sourceQuery(src: TreeSource) {
  return { source: toUri(src) };
}

// 文件清单全量一次拉取（扁平相对路径，git 管理的文件），前端用 lib/tree 组树
// 轮询仅对工作副本源生效——分支/提交源内容不可变，没必要周期性重取
export function useWorkbenchTree(path: string, src: TreeSource) {
  return useQuery({
    queryKey: ['workbench', 'tree', path, toUri(src)],
    queryFn: () => apiGet('/api/workbench/tree', { path, ...sourceQuery(src) }),
    enabled: path !== '' && !!src,
    refetchInterval: src?.type === 'worktree' ? 30_000 : false,
  });
}

export function useWorkbenchFile(path: string, src: TreeSource, file: string) {
  return useQuery({
    queryKey: ['workbench', 'file', path, toUri(src), file],
    queryFn: () => apiGet('/api/workbench/file', { path, ...sourceQuery(src), file }),
    enabled: path !== '' && !!src && file !== '',
  });
}

export function saveWorkbenchFile(path: string, src: TreeSource, file: string, content: string) {
  return apiPost('/api/workbench/file/save', { path, ...sourceQuery(src), file, content });
}

export type DiffEntry = components['schemas']['DiffEntry'];
export type DiffTreesResult = components['schemas']['DiffTreesResult'];
export type FileDiffResult = components['schemas']['FileDiffResult'];
export type Hunk = components['schemas']['Hunk'];

// 过滤（路径搜索）在 diff 面板内做（变更清单一次全量返回）；ignored 文件不在产品范围（默认过滤）
export function useWorkbenchDiff(path: string, base: TreeSource, current: TreeSource) {
  return useQuery({
    queryKey: ['workbench', 'diff', path, toUri(base), toUri(current)],
    queryFn: () =>
      apiGet('/api/workbench/diff', {
        path,
        base: toUri(base),
        current: toUri(current),
      }),
    enabled: path !== '' && base.id !== '' && current.id !== '',
    staleTime: 0, // 对比结果实时算
  });
}

// base 为 null 时后端按「相对基准」对比（worktree vs HEAD、ref/commit vs 父提交，同 /changes）；
// baseFile 为基准侧路径（rename 条目与当前侧不同，未改内容的 rename 两侧字节相同应显示一致）；
// enabled 供面板按内容模式懒拉
export function useWorkbenchFileDiff(
  path: string,
  base: TreeSource | null,
  current: TreeSource,
  file: string,
  enabled = true,
  baseFile = '',
) {
  return useQuery({
    queryKey: ['workbench', 'fileDiff', path, base ? toUri(base) : 'auto', toUri(current), file, baseFile],
    queryFn: () =>
      apiGet('/api/workbench/file-diff', {
        path,
        ...(base ? { base: toUri(base) } : {}),
        current: toUri(current),
        file,
        ...(baseFile ? { baseFile } : {}),
      }),
    enabled: enabled && path !== '' && file !== '' && current.id !== '',
    staleTime: 0,
  });
}

// 差异模式：源相对上一版本的变更文件（commit/ref vs 父提交；worktree vs HEAD）
export function useWorkbenchChanges(path: string, src: TreeSource, enabled: boolean) {
  return useQuery({
    queryKey: ['workbench', 'changes', path, toUri(src)],
    queryFn: () => apiGet('/api/workbench/changes', { path, ...sourceQuery(src) }),
    enabled: enabled && path !== '' && !!src,
    staleTime: 0,
    refetchInterval: src?.type === 'worktree' ? 30_000 : false,
  });
}

// --- worktree / 分支写侧（提案 1031）---
// 写成功后统一失效 workbench 全部查询（副本/refs/commits/tree 都可能变化）+
// 项目列表（后端已定向刷新 gitcache 快照，前端重取即可见新目标）

function useInvalidateWorkbench(path: string) {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: ['workbench'] });
    void qc.invalidateQueries({ queryKey: ['project', 'list'] });
    void path;
  };
}

// 新增 worktree：branch/commitish/targetPath 均可空（服务端定形态与预填路径）
export function useWorktreeAdd(path: string) {
  const invalidate = useInvalidateWorkbench(path);
  return useMutation({
    mutationFn: (input: { branch?: string; commitish?: string; targetPath?: string }) =>
      apiPost('/api/workbench/worktree/add', { path, ...input }),
    onSuccess: invalidate,
  });
}

// 删除 worktree：非 force 被预检拒绝时返回 denied=true + reasons（由 UI 二次确认升级 force）
export function useWorktreeRemove(path: string) {
  const invalidate = useInvalidateWorkbench(path);
  return useMutation({
    mutationFn: (input: { targetPath: string; force?: boolean }) =>
      apiPost('/api/workbench/worktree/remove', { path, ...input }),
    onSuccess: invalidate,
  });
}

// 重置 worktree 到指定分支/commit：hard=true 即 --hard（丢弃暂存区与工作区改动，UI 负责二次确认）
export function useWorktreeReset(path: string) {
  const invalidate = useInvalidateWorkbench(path);
  return useMutation({
    mutationFn: (input: { target: string; hard?: boolean }) =>
      apiPost('/api/workbench/worktree/reset', { path, ...input }),
    onSuccess: invalidate,
  });
}

// 新建本地分支（不检出——「建分支并切过去」走 worktree 新建）
export function useBranchAdd(path: string) {
  const invalidate = useInvalidateWorkbench(path);
  return useMutation({
    mutationFn: (input: { branch: string; commitish?: string }) =>
      apiPost('/api/workbench/branch/add', { path, ...input }),
    onSuccess: invalidate,
  });
}

export function useBranchDelete(path: string) {
  const invalidate = useInvalidateWorkbench(path);
  return useMutation({
    mutationFn: (input: { branch: string; force?: boolean }) =>
      apiPost('/api/workbench/branch/delete', { path, ...input }),
    onSuccess: invalidate,
  });
}

export type WorktreeCreated = components['schemas']['WorktreeCreated'];
export type WorktreeRemoveResult = components['schemas']['WorktreeRemoveResult'];
