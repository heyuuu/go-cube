import { useCallback, useEffect, useState } from 'react';

import { PANEL_ORDER, type PanelId } from './panels/registry';

// 工作台布局（提案 1015）：主区面板槽位列表，横向分栏。
// 布局存 localStorage（个人偏好，不进 URL——URL 管选中态，是面板间总线）。
// 分享 URL 时接收方按自己本地布局呈现选中态（总纲承诺）。

const LAYOUT_KEY = 'cube.workbench.layout';
const DEFAULT_LAYOUT: PanelId[] = ['auto'];
const MAX_SLOTS = 4; // 单实例约束下最多即全部四种

function loadLayout(): PanelId[] {
  try {
    const raw = localStorage.getItem(LAYOUT_KEY);
    if (!raw) return DEFAULT_LAYOUT;
    const arr = JSON.parse(raw);
    if (!Array.isArray(arr) || arr.length === 0) return DEFAULT_LAYOUT;
    const valid = arr.filter((p): p is PanelId => typeof p === 'string' && PANEL_ORDER.includes(p as PanelId));
    return valid.length > 0 ? dedupe(valid) : DEFAULT_LAYOUT;
  } catch {
    return DEFAULT_LAYOUT;
  }
}

function dedupe(list: PanelId[]): PanelId[] {
  return [...new Set(list)];
}

export function useWorkbenchLayout() {
  // 纯浏览器 SPA 无 SSR，localStorage 首渲染直接可用（同 md 页主题读取方式）
  const [slots, setSlots] = useState<PanelId[]>(() => loadLayout());

  useEffect(() => {
    if (slots) localStorage.setItem(LAYOUT_KEY, JSON.stringify(slots));
  }, [slots]);

  const addPanel = useCallback((id: PanelId) => {
    setSlots((cur) => {
      const base = cur ?? DEFAULT_LAYOUT;
      if (base.includes(id) || base.length >= MAX_SLOTS) return base;
      return [...base, id];
    });
  }, []);

  const removePanel = useCallback((id: PanelId) => {
    setSlots((cur) => {
      const base = cur ?? DEFAULT_LAYOUT;
      const next = base.filter((p) => p !== id);
      return next.length > 0 ? next : base; // 至少保留一个槽位
    });
  }, []);

  const resetLayout = useCallback(() => setSlots(DEFAULT_LAYOUT), []);

  return { slots, addPanel, removePanel, resetLayout };
}
