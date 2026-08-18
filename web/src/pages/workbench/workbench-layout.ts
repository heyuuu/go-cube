import { useCallback, useEffect, useState } from 'react';

import { PANEL_ORDER, type PanelId } from './panels/registry';

// 工作台布局（提案 1015 + 增强）：主区面板槽位列表横向分栏。
// slots = 面板顺序（标题栏拖拽排序），sizes = 各槽 flexGrow 比例（分隔条拖拽调宽）。
// 布局存 localStorage（个人偏好，不进 URL——URL 管选中态，是面板间总线）。
// 分享 URL 时接收方按自己本地布局呈现选中态（总纲承诺）。
// 兼容旧存储格式（纯 PanelId 数组）：无 sizes 时按等比归一。

const LAYOUT_KEY = 'cube.workbench.layout';
const DEFAULT_SLOTS: PanelId[] = ['auto'];
const MAX_SLOTS = 4; // 单实例约束下最多即全部四种

export type WorkbenchLayoutState = {
  slots: PanelId[];
  sizes: number[];
};

function normalizeSizes(slots: PanelId[], sizes: unknown): number[] {
  if (!Array.isArray(sizes)) return slots.map(() => 1);
  const picked = slots.map((_, i) => {
    const v = sizes[i];
    return typeof v === 'number' && v > 0 ? v : 1;
  });
  return picked.some((v) => v > 0) ? picked : slots.map(() => 1);
}

function loadLayout(): WorkbenchLayoutState {
  const fallback = (): WorkbenchLayoutState => ({
    slots: DEFAULT_SLOTS,
    sizes: normalizeSizes(DEFAULT_SLOTS, null),
  });
  try {
    const raw = localStorage.getItem(LAYOUT_KEY);
    if (!raw) return fallback();
    const parsed: unknown = JSON.parse(raw);
    const arr: unknown = Array.isArray(parsed) ? parsed : (parsed as { slots?: unknown })?.slots;
    if (!Array.isArray(arr) || arr.length === 0) return fallback();
    const valid = arr.filter((p): p is PanelId => typeof p === 'string' && PANEL_ORDER.includes(p as PanelId));
    if (valid.length === 0) return fallback();
    const slots = [...new Set(valid)];
    const sizes = Array.isArray(parsed) ? null : (parsed as { sizes?: unknown }).sizes;
    return { slots, sizes: normalizeSizes(slots, sizes) };
  } catch {
    return fallback();
  }
}

export function useWorkbenchLayout() {
  // 纯浏览器 SPA 无 SSR，localStorage 首渲染直接可用（同 md 页主题读取方式）
  const [state, setState] = useState<WorkbenchLayoutState>(() => loadLayout());

  useEffect(() => {
    localStorage.setItem(LAYOUT_KEY, JSON.stringify(state));
  }, [state]);

  const addPanel = useCallback((id: PanelId) => {
    setState((cur) => {
      if (cur.slots.includes(id) || cur.slots.length >= MAX_SLOTS) return cur;
      return { slots: [...cur.slots, id], sizes: [...cur.sizes, 1] };
    });
  }, []);

  const removePanel = useCallback((id: PanelId) => {
    setState((cur) => {
      const idx = cur.slots.indexOf(id);
      if (idx < 0 || cur.slots.length <= 1) return cur; // 至少保留一个槽位
      return {
        slots: cur.slots.filter((p) => p !== id),
        sizes: cur.sizes.filter((_, i) => i !== idx),
      };
    });
  }, []);

  // 拖拽排序：把 from 槽移动到 to 槽的位置
  const reorderPanel = useCallback((from: number, to: number) => {
    setState((cur) => {
      if (from === to || from < 0 || to < 0 || from >= cur.slots.length || to >= cur.slots.length) return cur;
      const slots = [...cur.slots];
      const sizes = [...cur.sizes];
      const [slot] = slots.splice(from, 1);
      const [size] = sizes.splice(from, 1);
      slots.splice(to, 0, slot);
      sizes.splice(to, 0, size);
      return { slots, sizes };
    });
  }, []);

  // 分隔条拖拽：delta 为相邻两槽 flexGrow 比例的转移量
  const resizePanels = useCallback((index: number, delta: number) => {
    setState((cur) => {
      if (index < 0 || index + 1 >= cur.sizes.length) return cur;
      const min = 0.15; // 单槽最小比例，防止拖成 0 宽
      const next = cur.sizes[index] + delta;
      const other = cur.sizes[index + 1] - delta;
      if (next < min || other < min) return cur;
      const sizes = [...cur.sizes];
      sizes[index] = next;
      sizes[index + 1] = other;
      return { slots: cur.slots, sizes };
    });
  }, []);

  const resetLayout = useCallback(
    () => setState({ slots: DEFAULT_SLOTS, sizes: normalizeSizes(DEFAULT_SLOTS, null) }),
    [],
  );

  return { ...state, addPanel, removePanel, reorderPanel, resizePanels, resetLayout };
}
