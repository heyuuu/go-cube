import { useEffect, useState } from 'react';

// 终端面板偏好（字体/字号/高度），localStorage 持久化，写法同 useTreePanePrefs。

const PREFS_KEY = 'workbench.terminal';

export const MIN_FONT_SIZE = 10;
export const MAX_FONT_SIZE = 20;
export const MIN_HEIGHT = 160;
export const MAX_HEIGHT = 600;

// 字体预设：值为 CSS font-family 回退栈——特殊字符（powerline 等）靠浏览器
// 逐级回退兜底，未安装的字体自动落到下一级，最后必有 monospace。
export const FONT_PRESETS = [
  { id: 'system', label: '系统默认', stack: "Menlo, Monaco, 'Courier New', monospace" },
  { id: 'menlo', label: 'Menlo', stack: 'Menlo, monospace' },
  { id: 'monaco', label: 'Monaco', stack: 'Monaco, Menlo, monospace' },
  { id: 'sfmono', label: 'SF Mono', stack: "'SF Mono', Menlo, monospace" },
  { id: 'jetbrains', label: 'JetBrains Mono', stack: "'JetBrains Mono', Menlo, monospace" },
] as const;

export type FontPresetId = (typeof FONT_PRESETS)[number]['id'];

function loadFontPresetId(): FontPresetId {
  const saved = localStorage.getItem(`${PREFS_KEY}.font`);
  return FONT_PRESETS.some((p) => p.id === saved) ? (saved as FontPresetId) : 'system';
}

export function useTerminalPrefs() {
  const [fontId, setFontId] = useState<FontPresetId>(loadFontPresetId);
  const [fontSize, setFontSize] = useState(() => {
    const v = Number(localStorage.getItem(`${PREFS_KEY}.fontSize`));
    return Number.isFinite(v) && v >= MIN_FONT_SIZE && v <= MAX_FONT_SIZE ? v : 12;
  });
  const [height, setHeight] = useState(() => {
    const v = Number(localStorage.getItem(`${PREFS_KEY}.height`));
    return Number.isFinite(v) && v >= MIN_HEIGHT && v <= MAX_HEIGHT ? v : 256;
  });

  useEffect(() => localStorage.setItem(`${PREFS_KEY}.font`, fontId), [fontId]);
  useEffect(() => localStorage.setItem(`${PREFS_KEY}.fontSize`, String(fontSize)), [fontSize]);
  useEffect(() => localStorage.setItem(`${PREFS_KEY}.height`, String(height)), [height]);

  const fontFamily = (FONT_PRESETS.find((p) => p.id === fontId) ?? FONT_PRESETS[0]).stack;
  return { fontId, setFontId, fontFamily, fontSize, setFontSize, height, setHeight };
}
