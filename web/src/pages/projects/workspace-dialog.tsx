import { useQueryClient } from '@tanstack/react-query';
// workspace 声明编辑弹窗（1030）：从项目详情抽屉直达（cube.json 是仓库级事实，
// 入口必须在项目上下文内，不放全局 settings）。轻量自绘 overlay（同 ConfirmDialog 模式）。
import { useState } from 'react';

import type { Project } from '@/api/client';
import { DialogShell } from '@/components/dialog-shell';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useWorkspaceSave, useWorkspaceState } from '@/queries/project';

type Member = { name: string; path: string };

export function WorkspaceDialog({ project, onClose }: { project: Project; onClose: () => void }) {
  const qc = useQueryClient();
  const state = useWorkspaceState(project.path);
  const save = useWorkspaceSave();

  const [members, setMembers] = useState<Member[]>([]);

  // 打开时以显式声明初始化；无显式声明（纯探测生效）以探测候选初始化——
  // 两种情况「固化」的起点都是机器推导结果，再由用户裁剪。
  // 数据异步到达，用「渲染期间调整 state」模式同步（React 推荐，优于 effect）
  const declared = state.data?.declaredSet ? state.data?.declared : state.data?.detected;
  const [prevDeclared, setPrevDeclared] = useState(declared);
  if (declared !== prevDeclared) {
    setPrevDeclared(declared);
    setMembers((declared ?? []).map((m) => ({ ...m })));
  }

  function setAt(i: number, patch: Partial<Member>) {
    setMembers((prev) => prev.map((m, j) => (j === i ? { ...m, ...patch } : m)));
  }

  // 导入探测候选：追加当前清单里没有的条目
  function importDetected() {
    const detected = state.data?.detected ?? [];
    setMembers((prev) => {
      const known = new Set(prev.map((m) => m.path));
      return [...prev, ...detected.filter((d) => !known.has(d.path)).map((d) => ({ ...d }))];
    });
  }

  function submit() {
    save.mutate(
      { path: project.path, workspaces: members },
      {
        onSuccess: async () => {
          await qc.invalidateQueries({ queryKey: ['project', 'list'] });
          await qc.invalidateQueries({ queryKey: ['project', 'workspace', project.path] });
          onClose();
        },
      },
    );
  }

  return (
    <DialogShell title="workspace 声明" onClose={onClose} className="flex max-h-[80vh] max-w-lg flex-col">
        <div className="mt-1 text-xs text-muted-foreground">
          写入项目内 .cube/cube.json（进 git）。显式声明优先生效；未声明时按探测规则自动生效。
        </div>

        {state.isLoading ? (
          <div className="py-6 text-center text-xs text-muted-foreground">加载中…</div>
        ) : (
          <>
            <div className="mt-3 flex-1 space-y-2 overflow-y-auto">
              {members.length === 0 && (
                <div className="py-2 text-xs text-muted-foreground">
                  暂无成员
                  {state.data?.detected?.length ? '，可点下方「导入探测候选」' : '（未探测到标准 monorepo 声明）'}
                </div>
              )}
              {members.map((m, i) => (
                <div key={i} className="flex items-center gap-2">
                  <Input
                    className="h-8 w-32 shrink-0"
                    value={m.name}
                    placeholder="名称"
                    onChange={(e) => setAt(i, { name: e.target.value })}
                  />
                  <Input
                    className="h-8 flex-1 font-mono text-xs"
                    value={m.path}
                    placeholder="相对项目根路径"
                    onChange={(e) => setAt(i, { path: e.target.value })}
                  />
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setMembers((prev) => prev.filter((_, j) => j !== i))}
                  >
                    移除
                  </Button>
                </div>
              ))}
            </div>

            <div className="mt-3 flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setMembers((prev) => [...prev, { name: '', path: '' }])}
              >
                添加
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={importDetected}
                disabled={!state.data?.detected?.length}
                title="追加标准 monorepo 声明文件探测到的候选（去重）"
              >
                导入探测候选
              </Button>
              {save.isError && <span className="text-xs text-destructive">{save.error.message}</span>}
            </div>

            <div className="mt-4 flex justify-end gap-2">
              <Button variant="outline" size="sm" onClick={onClose}>
                取消
              </Button>
              <Button
                size="sm"
                onClick={submit}
                disabled={save.isPending || members.length === 0 || members.some((m) => !m.path)}
              >
                保存
              </Button>
            </div>
          </>
        )}
    </DialogShell>
  );
}
