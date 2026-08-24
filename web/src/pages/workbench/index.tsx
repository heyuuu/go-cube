import { ChevronsLeft, ChevronsRight, GripVertical, LayoutGrid, Plus, RotateCcw, X } from 'lucide-react';
import { Fragment, useRef } from 'react';
import { useSearchParams } from 'react-router';

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useWorkbenchInfo } from '@/queries/workbench';

import { ContentViewPanel } from './panels/content-view-panel';
import { GitTreePanel } from './panels/git-tree-panel';
import { ContentPanelPlaceholder } from './panels/placeholders';
import { PANEL_REGISTRY, PANEL_ORDER, type PanelId } from './panels/registry';
import { TerminalPanel } from './panels/terminal-panel';
import { readWorkbenchParams, writePathParam } from './params';
import { PathEntry } from './path-entry';
import { ProjectSwitcher } from './project-switcher';
import { PanelSplitter } from './splitter';
import { useWorkbenchLayout } from './workbench-layout';

// 工作台页面（1010 基座 → 1011 选择 → 1015 面板组装）：以任意本机 git 目录为输入，
// 聚合 git 可视化 / 代码阅读 / diff / PTY。挂全局壳（1023），Layout 按本前缀切铺满型 main。
// URL 是面板间唯一总线（path + 选中态 source/left/right/file，见 params.ts）；
// 布局（面板槽位组合）存 localStorage，属个人偏好不进 URL。
// 终端固定底部抽屉，不进主区布局；主区面板同类型单实例（多实例留待后续）。
export function WorkbenchPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const params = readWorkbenchParams(searchParams);
  const layout = useWorkbenchLayout();
  const slotsRef = useRef<HTMLDivElement>(null);
  const dragFrom = useRef<number | null>(null);
  // path 合法性守门：非 git 目录退回入口页展示错误（与 git 树面板共用同一 query key，
  // 校验通过后面板直接复用缓存）
  const info = useWorkbenchInfo(params.path);

  const submitPath = (value: string) => {
    const next = new URLSearchParams(searchParams);
    if (value) {
      writePathParam(next, value);
    } else {
      next.delete('path');
    }
    setSearchParams(next, { replace: true });
  };

  // 切项目（1023）：清空旧仓库的选中态参数（source/left/right 属于旧仓库，留着是脏数据），
  // 保留 panel 等与仓库无关的个人偏好参数由各面板自行管理（目前无此类参数，仅 path）
  const switchProject = (path: string) => setSearchParams({ path });

  if (!params.path || info.isError) {
    return (
      <main className="h-full bg-background px-4">
        <PathEntry
          initial={params.path}
          initialError={
            info.isError ? String(info.error instanceof Error ? info.error.message : info.error) : undefined
          }
          onSubmit={submitPath}
        />
      </main>
    );
  }

  const hasSelection = params.current;

  const renderPanel = (id: PanelId) => {
    switch (id) {
      case 'git-tree':
        return <GitTreePanel params={params} />;
      case 'content':
        return hasSelection ? (
          <ContentViewPanel params={params} />
        ) : (
          <ContentPanelPlaceholder title="从 Git 树面板选择一个目标开始（cmd/ctrl 点第二个目标 = 对比）" />
        );
    }
  };

  const addable = PANEL_ORDER.filter((p) => !layout.slots.includes(p));

  return (
    <div className="flex h-full flex-col bg-background text-foreground">
      <header className="flex shrink-0 items-center gap-2 border-b border-border px-3 py-2">
        <span className="text-xs font-semibold tracking-wide">工作台</span>
        {/* 项目下拉（1023）：沉浸场景下不退回列表直接横跳其他项目；非项目目录（任意 git 目录入口）时菜单仍可用 */}
        <ProjectSwitcher current={params.path} onSwitch={switchProject} />
        <div className="ml-auto flex items-center gap-1">
          <DropdownMenu>
            <DropdownMenuTrigger
              className="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground"
              render={
                <button type="button">
                  <LayoutGrid className="mr-1 inline size-3.5" />
                  面板
                  <Plus className="ml-0.5 inline size-3" />
                </button>
              }
            />
            <DropdownMenuContent align="end">
              {addable.length === 0 ? (
                <div className="px-2 py-1.5 text-xs text-muted-foreground">所有面板均已打开（同类型单实例）</div>
              ) : null}
              {PANEL_ORDER.map((id) => {
                const item = PANEL_REGISTRY[id];
                return (
                  <DropdownMenuItem key={id} disabled={!addable.includes(id)} onClick={() => layout.addPanel(id)}>
                    <item.icon className="mr-1.5 size-3.5" />
                    {item.label}
                  </DropdownMenuItem>
                );
              })}
            </DropdownMenuContent>
          </DropdownMenu>
          <button
            type="button"
            title="恢复默认布局"
            className="rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground"
            onClick={layout.resetLayout}
          >
            <RotateCcw className="size-3.5" />
          </button>
          <button
            type="button"
            className="rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground"
            onClick={() => submitPath('')}
          >
            更换目录
          </button>
        </div>
      </header>
      <div ref={slotsRef} className="flex min-h-0 flex-1">
        {layout.slots.map((id, i) => {
          const item = PANEL_REGISTRY[id];
          const slim = layout.slim.includes(id);
          return (
            <Fragment key={id}>
              {slim ? (
                // 收窄态：细条整条都是展开按钮（竖排标签保持可辨识），宽度固定不参与 flexGrow
                <section className="flex w-10 shrink-0 flex-col items-center border-r border-border bg-muted/30">
                  <button
                    type="button"
                    title={`${item.label}：展开`}
                    aria-label={`${item.label}：展开`}
                    className="flex h-full w-full flex-col items-center gap-2 py-2 text-muted-foreground hover:text-accent-foreground"
                    onClick={() => layout.toggleSlim(id)}
                  >
                    <ChevronsRight className="size-3.5" />
                    <span className="text-[11px] [writing-mode:vertical-rl]">{item.label}</span>
                  </button>
                </section>
              ) : (
                <section className="flex min-w-48 flex-col" style={{ flexGrow: layout.sizes[i] ?? 1, flexBasis: 0 }}>
                  <div
                    draggable
                    onDragStart={() => {
                      dragFrom.current = i;
                    }}
                    onDragOver={(e) => {
                      if (dragFrom.current === null || dragFrom.current === i) return;
                      e.preventDefault();
                      layout.reorderPanel(dragFrom.current, i);
                      dragFrom.current = i;
                    }}
                    className="flex h-7 shrink-0 cursor-grab items-center gap-1.5 border-b border-border bg-muted/30 px-2 text-[11px] text-muted-foreground active:cursor-grabbing"
                    title="拖拽调整面板顺序"
                    onDoubleClick={() => layout.toggleSlim(id)}
                  >
                    <GripVertical className="size-3 shrink-0 opacity-50" />
                    <item.icon className="size-3.5" />
                    {item.label}
                    <button
                      type="button"
                      title="收窄面板（再点展开）"
                      className="ml-auto rounded p-0.5 hover:bg-accent hover:text-accent-foreground"
                      onClick={() => layout.toggleSlim(id)}
                      onDoubleClick={(e) => e.stopPropagation()}
                    >
                      <ChevronsLeft className="size-3" />
                    </button>
                    <button
                      type="button"
                      title="移除面板"
                      className="rounded p-0.5 hover:bg-accent hover:text-accent-foreground"
                      onClick={() => layout.removePanel(id)}
                      onDoubleClick={(e) => e.stopPropagation()}
                    >
                      <X className="size-3" />
                    </button>
                  </div>
                  <div className="min-h-0 flex-1 overflow-hidden">{renderPanel(id)}</div>
                </section>
              )}
              {i < layout.slots.length - 1 ? (
                <PanelSplitter
                  onDelta={(dx) => {
                    // 像素位移换算为 flexGrow 比例：总宽 / 总比例 = 单位比例的像素数
                    const pxPerGrow = (slotsRef.current?.clientWidth ?? 0) / layout.sizes.reduce((a, b) => a + b, 0);
                    if (pxPerGrow > 0) layout.resizePanels(i, dx / pxPerGrow);
                  }}
                />
              ) : null}
            </Fragment>
          );
        })}
      </div>
      <TerminalPanel path={params.path} />
    </div>
  );
}
