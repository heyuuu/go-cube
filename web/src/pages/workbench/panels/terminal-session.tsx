import { FitAddon } from '@xterm/addon-fit';
import { Terminal } from '@xterm/xterm';
import { Minus, RotateCw, X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

import { MAX_FONT_SIZE, MIN_FONT_SIZE } from './use-terminal-prefs';

import '@xterm/xterm/css/xterm.css';

// PTY 会话（提案 1014）：挂载即建立 WebSocket、卸载即断开杀进程，
// 重连 = 新会话（不做保留）。协议帧：input/resize（出）、output/exit（入），
// 与后端 pty.go 对应。字体/字号变化走 term.options 热更新，不重建会话。
// 终端内快捷键：cmd+D 在本实例后分屏、ctrl+shift+W 关闭本实例（经回调上抛面板处理）；
// pty 退出（exit 帧）经 onExit 上抛，由面板移除本实例。
// 折叠面板不卸载本组件——会话由面板常驻渲染保持。

type PtyPhase = 'connecting' | 'open' | 'exited' | 'error';

const PHASE_TEXT: Record<PtyPhase, string> = {
  connecting: '连接中…',
  open: '已连接',
  exited: '已退出',
  error: '连接失败',
};

export function TerminalSession({
  path,
  fontFamily,
  fontSize,
  onFontSizeChange,
  onFocused,
  onSplit,
  onClose,
  onExit,
}: {
  path: string;
  fontFamily: string;
  fontSize: number;
  onFontSizeChange?: (delta: number) => void;
  onFocused: () => void;
  onSplit: () => void;
  onClose: () => void;
  onExit: () => void;
}) {
  const [phase, setPhase] = useState<PtyPhase>('connecting');
  const [exitCode, setExitCode] = useState<number | null>(null);
  const [errorMsg, setErrorMsg] = useState('');
  const [seq, setSeq] = useState(0);
  const hostRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  // term 实例在会话 effect 内创建，字号 effect 需要等它就绪后再跑一次
  const [termReady, setTermReady] = useState(0);
  // 会话不随字体变化重建（走热更新），effect 里经 ref 取初值，避免进依赖数组
  const fontRef = useRef({ fontFamily, fontSize });
  useEffect(() => {
    fontRef.current = { fontFamily, fontSize };
  }, [fontFamily, fontSize]);
  // 快捷键 / 焦点 / 退出回调经 ref 进事件处理器，不参与会话 effect 依赖
  const handlers = useRef({ onFocused, onSplit, onClose, onExit });
  useEffect(() => {
    handlers.current = { onFocused, onSplit, onClose, onExit };
  });
  // 每个会话周期（seq）只上抛一次退出，回调身份变化不重复触发
  const exitFired = useRef(false);
  useEffect(() => {
    exitFired.current = false;
  }, [seq]);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    setPhase('connecting');
    setExitCode(null);
    setErrorMsg('');

    const term = new Terminal({ ...fontRef.current, cursorBlink: true });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);
    fit.fit();
    termRef.current = term;
    fitRef.current = fit;
    setTermReady((n) => n + 1);

    // 终端内快捷键：cmd+D 分屏；ctrl+shift+W 关屏（cmd+W 是浏览器保留键，不绑定）。
    // preventDefault 阻断浏览器默认（cmd+D 收藏页面）、stopPropagation 阻断全局快捷键
    // （use-theme 的 ⌘D 暗黑切换）
    term.attachCustomKeyEventHandler((ev) => {
      if (ev.type !== 'keydown') return true;
      const key = ev.key.toLowerCase();
      const plainCmdD = ev.metaKey && !ev.ctrlKey && !ev.altKey && !ev.shiftKey && key === 'd';
      const ctrlShiftW = ev.ctrlKey && ev.shiftKey && !ev.metaKey && !ev.altKey && key === 'w';
      if (plainCmdD || ctrlShiftW) {
        ev.preventDefault();
        ev.stopPropagation();
        if (plainCmdD) handlers.current.onSplit();
        else handlers.current.onClose();
        return false;
      }
      return true;
    });
    // 焦点落在本实例（点击 / term.focus）即成为「当前实例」，供分屏插入位置用
    const onFocusIn = () => handlers.current.onFocused();
    host.addEventListener('focusin', onFocusIn);
    host.addEventListener('mousedown', onFocusIn);

    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${proto}://${location.host}/api/workbench/pty?path=${encodeURIComponent(path)}&cols=${term.cols}&rows=${term.rows}`;
    const ws = new WebSocket(url);
    ws.binaryType = 'arraybuffer';

    ws.onopen = () => setPhase('open');
    ws.onerror = () => {
      setPhase('error');
      setErrorMsg('WebSocket 连接失败');
    };
    ws.onclose = () => {
      // 函数式更新避免覆盖已置的终态（error / exit）
      setPhase((p) => (p === 'connecting' || p === 'open' ? 'exited' : p));
    };
    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(String(ev.data));
        if (msg.type === 'output') term.write(msg.data);
        if (msg.type === 'exit') {
          setExitCode(msg.code ?? 0);
          setPhase('exited');
        }
      } catch {
        /* 非 JSON 帧忽略 */
      }
    };
    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'input', data }));
      }
    });
    const sendResize = () => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
      }
    };
    const resizeObserver = new ResizeObserver(() => {
      // display:none 的隐藏 tab 量出 0 尺寸，跳过以免把 pty 缩没
      if (host.clientWidth > 0 && host.clientHeight > 0) {
        fit.fit();
        sendResize();
      }
    });
    resizeObserver.observe(host);
    term.focus();

    wsRef.current = ws;
    return () => {
      host.removeEventListener('focusin', onFocusIn);
      host.removeEventListener('mousedown', onFocusIn);
      resizeObserver.disconnect();
      wsRef.current = null;
      ws.close();
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
    };
    // 会话只随 seq / path 重建；字号字体经热更新生效
  }, [path, seq]);

  useEffect(() => {
    const term = termRef.current;
    if (!term) return;
    term.options.fontFamily = fontFamily;
    term.options.fontSize = fontSize;
    fitRef.current?.fit();
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
    }
  }, [fontFamily, fontSize, termReady]);

  // pty 退出（exit 帧 / 断连）→ 面板移除本实例；连接失败不在此列（保留重试入口）
  useEffect(() => {
    if (phase === 'exited' && !exitFired.current) {
      exitFired.current = true;
      handlers.current.onExit();
    }
  }, [phase]);

  const restart = () => setSeq((n) => n + 1);

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col bg-black">
      <div className="flex h-6 shrink-0 items-center gap-2 px-2 text-[10px] text-muted-foreground">
        <span className={cn(phase === 'open' && 'text-green-500', phase === 'error' && 'text-destructive')}>
          {phase === 'exited' && exitCode !== null ? `已退出（${exitCode}）` : PHASE_TEXT[phase]}
        </span>
        <div className="ml-auto flex items-center gap-0.5">
          {onFontSizeChange && (
            <>
              <button
                type="button"
                title="减小字号"
                disabled={fontSize <= MIN_FONT_SIZE}
                className="rounded p-0.5 hover:bg-accent hover:text-accent-foreground disabled:opacity-30"
                onClick={() => onFontSizeChange(-1)}
              >
                <Minus className="size-3" />
              </button>
              <span className="w-6 text-center tabular-nums">{fontSize}</span>
              <button
                type="button"
                title="增大字号"
                disabled={fontSize >= MAX_FONT_SIZE}
                className="rounded p-0.5 hover:bg-accent hover:text-accent-foreground disabled:opacity-30"
                onClick={() => onFontSizeChange(1)}
              >
                <span className="text-[11px] font-bold">A</span>
              </button>
            </>
          )}
          <button
            type="button"
            title="重开会话"
            className="rounded p-0.5 hover:bg-accent hover:text-accent-foreground"
            onClick={restart}
          >
            <RotateCw className="size-3" />
          </button>
          <button
            type="button"
            title="关闭终端（ctrl+shift+W）"
            className="rounded p-0.5 hover:bg-accent hover:text-accent-foreground"
            onClick={onClose}
          >
            <X className="size-3" />
          </button>
        </div>
      </div>
      <div className="relative min-h-0 flex-1 px-2 pb-2">
        <div ref={hostRef} className="h-full" />
        {phase === 'error' && (
          <div className="absolute inset-0 flex items-center justify-center gap-3 bg-black/70 text-xs text-muted-foreground">
            {errorMsg}
            <Button variant="outline" size="sm" onClick={restart}>
              <RotateCw className="mr-1 size-3" />
              重新开始
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}
