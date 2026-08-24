import { ChevronDown, ChevronUp, Columns2, Plus, Settings2, TerminalSquare, X } from 'lucide-react';
import { Fragment, useRef, useState } from 'react';

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { PanelSplitter } from '@/pages/workbench/splitter';
import { cn } from '@/lib/utils';

import { TerminalSession } from './terminal-session';
import {
  FONT_PRESETS,
  MAX_FONT_SIZE,
  MAX_HEIGHT,
  MIN_FONT_SIZE,
  MIN_HEIGHT,
  useTerminalPrefs,
} from './use-terminal-prefs';

// PTY 终端面板（提案 1014）：底部抽屉。多终端 = tab 切换 + tab 内横向分屏（不限个数），
// 每个实例一条独立 WebSocket（后端按连接起 shell）。
// 生命周期：折叠不杀会话（内容常驻渲染、仅隐藏）；关实例 / 关 tab / 实例 pty 退出
// 才断开。实例空 → tab 自动关，tab 清空 → 面板自动折叠，展开时无 tab 自动新建一个。
// tab 结构不持久化，刷新即重来；字体/字号/高度走 localStorage 偏好。

type Pane = { id: number; flex: number };
type Tab = { id: number; panes: Pane[] };

let seq = 0;
const nextId = () => ++seq;
const newTab = (): Tab => ({ id: nextId(), panes: [{ id: nextId(), flex: 1 }] });

export function TerminalPanel({ path }: { path: string }) {
  const [expanded, setExpanded] = useState(false);
  const [tabs, setTabs] = useState<Tab[]>([]);
  const [activeTabId, setActiveTabId] = useState<number | null>(null);
  const [activePaneId, setActivePaneId] = useState<number | null>(null);
  const { fontId, setFontId, fontFamily, fontSize, setFontSize, height, setHeight } =
    useTerminalPrefs();
  const contentRef = useRef<HTMLDivElement>(null);

  // tab 清空 → 面板视为折叠（派生态，不做级联 setState）；active 指向已删 id 时回落首个 tab
  const open = expanded && tabs.length > 0;
  const activeTab = tabs.find((t) => t.id === activeTabId) ?? tabs[0];

  const toggleExpanded = () => {
    if (!open && tabs.length === 0) {
      const tab = newTab();
      setTabs([tab]);
      setActiveTabId(tab.id);
      setActivePaneId(tab.panes[0].id);
    }
    setExpanded(!open);
  };
  const stop = (e: React.MouseEvent) => e.stopPropagation();

  const addTab = () => {
    const tab = newTab();
    setTabs((ts) => [...ts, tab]);
    setActiveTabId(tab.id);
    setActivePaneId(tab.panes[0].id);
  };
  const closeTab = (id: number) => setTabs((ts) => ts.filter((t) => t.id !== id));

  // 分屏：新实例插在「当前实例」后；当前 tab 无当前实例（如失焦）则追加到末尾
  const splitPane = (tabId: number, afterPaneId: number | null) => {
    const paneId = nextId();
    setTabs((ts) =>
      ts.map((t) => {
        if (t.id !== tabId) return t;
        const idx = afterPaneId ? t.panes.findIndex((p) => p.id === afterPaneId) : -1;
        const pane = { id: paneId, flex: 1 };
        const panes =
          idx >= 0
            ? [...t.panes.slice(0, idx + 1), pane, ...t.panes.slice(idx + 1)]
            : [...t.panes, pane];
        return { ...t, panes };
      }),
    );
    setActiveTabId(tabId);
    setActivePaneId(paneId);
  };
  const removePane = (tabId: number, paneId: number) => {
    setTabs((ts) =>
      ts
        .map((t) => (t.id === tabId ? { ...t, panes: t.panes.filter((p) => p.id !== paneId) } : t))
        .filter((t) => t.panes.length > 0),
    );
  };
  // split 拖拽：按容器宽度把像素位移折算成相邻两 pane 的 flex 增量（此消彼长）
  const adjustFlex = (tabId: number, paneIdx: number, dx: number) => {
    const width = contentRef.current?.clientWidth ?? 0;
    if (width <= 0) return;
    setTabs((ts) =>
      ts.map((t) => {
        if (t.id !== tabId || paneIdx + 1 >= t.panes.length) return t;
        const panes = t.panes.map((p) => ({ ...p }));
        const total = panes.reduce((sum, p) => sum + p.flex, 0);
        const delta = (dx / width) * total;
        const a = panes[paneIdx].flex + delta;
        const b = panes[paneIdx + 1].flex - delta;
        // 每侧至少 10% 宽度，防止拖成 0
        const lo = total * 0.1;
        if (a < lo || b < lo) return t;
        panes[paneIdx].flex = a;
        panes[paneIdx + 1].flex = b;
        return { ...t, panes };
      }),
    );
  };
  const changeFontSize = (delta: number) =>
    setFontSize((v) => Math.min(Math.max(v + delta, MIN_FONT_SIZE), MAX_FONT_SIZE));

  return (
    <div
      className="relative flex shrink-0 flex-col border-t border-border"
      // 高度 +1 补偿 border-t：h-9 的 header 要在内容盒里完整放下（36px 内容 + 1px 边框），
      // 否则 header 溢出内容盒 1px，标题内容整体下沉、看起来不居中
      style={{ height: (open ? height : 36) + 1 }}
    >
      {open && (
        <div
          role="separator"
          aria-orientation="horizontal"
          className="absolute -mt-1 h-2 w-full cursor-row-resize"
          onPointerDown={(e) => {
            e.currentTarget.setPointerCapture(e.pointerId);
            e.preventDefault();
          }}
          onPointerMove={(e) => {
            if (!(e.buttons & 1)) return;
            // 向上拖（dy<0）增高；夹在偏好范围内，拖完由 hook 持久化
            setHeight((h) => Math.min(Math.max(h - e.movementY, MIN_HEIGHT), MAX_HEIGHT));
          }}
        />
      )}
      <div
        className="flex h-9 shrink-0 items-center gap-1 px-2 text-xs text-muted-foreground"
        onDoubleClick={toggleExpanded}
      >
        <button
          type="button"
          title={open ? '收起终端' : '展开终端'}
          className="rounded p-1 hover:bg-accent hover:text-accent-foreground"
          onClick={(e) => {
            stop(e);
            toggleExpanded();
          }}
          onDoubleClick={stop}
        >
          {open ? <ChevronDown className="size-3.5" /> : <ChevronUp className="size-3.5" />}
        </button>
        <TerminalSquare className="size-3.5" />
        <div className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
          {tabs.map((tab, i) => (
            <span
              key={tab.id}
              className={cn(
                'flex shrink-0 items-center gap-1 rounded px-2 py-0.5 text-[11px]',
                tab.id === activeTab?.id ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50',
              )}
              onClick={stop}
              onDoubleClick={stop}
            >
              <button type="button" onClick={() => setActiveTabId(tab.id)}>
                终端 {i + 1}
                {tab.panes.length > 1 && `（${tab.panes.length}）`}
              </button>
              <button
                type="button"
                title="关闭标签"
                className="rounded p-0.5 hover:bg-foreground/10"
                onClick={() => closeTab(tab.id)}
              >
                <X className="size-3" />
              </button>
            </span>
          ))}
          <button
            type="button"
            title="新建终端"
            className="shrink-0 rounded p-1 hover:bg-accent hover:text-accent-foreground"
            onClick={(e) => {
              stop(e);
              addTab();
            }}
            onDoubleClick={stop}
          >
            <Plus className="size-3.5" />
          </button>
        </div>
        <button
          type="button"
          title="分屏（在当前终端后新增一屏）"
          className="rounded p-1 hover:bg-accent hover:text-accent-foreground"
          onClick={(e) => {
            stop(e);
            if (activeTab) splitPane(activeTab.id, activePaneId);
          }}
          onDoubleClick={stop}
        >
          <Columns2 className="size-3.5" />
        </button>
        <div onClick={stop} onDoubleClick={stop}>
          <DropdownMenu>
            <DropdownMenuTrigger
              className="rounded p-1 hover:bg-accent hover:text-accent-foreground"
              title="终端设置"
            >
              <Settings2 className="size-3.5" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="min-w-56">
              <DropdownMenuRadioGroup
                value={fontId}
                onValueChange={(v) => setFontId(v as (typeof FONT_PRESETS)[number]['id'])}
              >
                {FONT_PRESETS.map((preset) => (
                  <DropdownMenuRadioItem key={preset.id} value={preset.id}>
                    {preset.label}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
              <DropdownMenuSeparator />
              <DropdownMenuItem disabled={fontSize <= MIN_FONT_SIZE} onClick={() => changeFontSize(-1)}>
                减小字号（当前 {fontSize}）
              </DropdownMenuItem>
              <DropdownMenuItem disabled={fontSize >= MAX_FONT_SIZE} onClick={() => changeFontSize(1)}>
                增大字号（当前 {fontSize}）
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
      {/* 折叠不卸载：内容常驻（隐藏），会话保持连接；tab 为空时整个内容区不渲染 */}
      {tabs.length > 0 && (
        <div
          ref={contentRef}
          className={cn('relative flex min-h-0 flex-1 flex-col', !open && 'hidden')}
        >
          {tabs.map((tab) => (
            <div
              key={tab.id}
              className={cn('flex min-h-0 flex-1', tab.id !== activeTab?.id && 'hidden')}
            >
              {tab.panes.map((pane, i) => (
                <Fragment key={pane.id}>
                  {i > 0 && <PanelSplitter onDelta={(dx) => adjustFlex(tab.id, i - 1, dx)} />}
                  <div className="flex min-w-0 flex-col" style={{ flex: `${pane.flex} 1 0%` }}>
                    <TerminalSession
                      path={path}
                      fontFamily={fontFamily}
                      fontSize={fontSize}
                      onFontSizeChange={changeFontSize}
                      onFocused={() => {
                        setActiveTabId(tab.id);
                        setActivePaneId(pane.id);
                      }}
                      onSplit={() => splitPane(tab.id, pane.id)}
                      onClose={() => removePane(tab.id, pane.id)}
                      onExit={() => removePane(tab.id, pane.id)}
                    />
                  </div>
                </Fragment>
              ))}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
