import { FitAddon } from '@xterm/addon-fit';
import { Terminal } from '@xterm/xterm';
import { ChevronDown, ChevronUp, RotateCw, TerminalSquare } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

import '@xterm/xterm/css/xterm.css';

// PTY 终端面板（提案 1014）：底部抽屉，默认收起；展开时建立 WebSocket。
// 生命周期（提案定稿）：一个展开周期 = 一个会话；收起/关页即断开杀进程，
// 重连 = 新会话（不做保留）。不进 URL 状态。
// 协议帧：input/resize（出）、output/exit（入），与后端 pty.go 对应。

type PtyPhase = 'closed' | 'connecting' | 'open' | 'exited' | 'error';

export function TerminalPanel({ path }: { path: string }) {
  const [expanded, setExpanded] = useState(false);
  const [phase, setPhase] = useState<PtyPhase>('closed');
  const [exitCode, setExitCode] = useState<number | null>(null);
  const [errorMsg, setErrorMsg] = useState('');
  const [sessionSeq, setSessionSeq] = useState(0);
  const termRef = useRef<HTMLDivElement>(null);

  // 建立 WebSocket 会话（expanded 且有 sessionSeq 时）；断开由 cleanup 负责
  useEffect(() => {
    if (!expanded || !termRef.current || sessionSeq === 0) return;
    setPhase('connecting');

    const term = new Terminal({ fontSize: 12, cursorBlink: true, convertEol: false });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(termRef.current);
    fit.fit();

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
      // 函数式更新避免读外部 phase（收起时已置 closed，不覆盖）
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
    const resizeObserver = new ResizeObserver(() => {
      fit.fit();
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
      }
    });
    resizeObserver.observe(termRef.current);
    term.focus();

    return () => {
      resizeObserver.disconnect();
      // 收起抽屉 = 断开 = 后端杀进程（提案生命周期决策）
      ws.close();
      term.dispose();
    };
    // 会话只随 sessionSeq / path 重建；phase 用 ref 供 onclose 读取
  }, [expanded, sessionSeq, path]);

  const restart = () => {
    setExitCode(null);
    setErrorMsg('');
    setSessionSeq((n) => n + 1);
  };

  return (
    <div className={cn('flex shrink-0 flex-col border-t border-border transition-all', expanded ? 'h-64' : 'h-9')}>
      <button
        type="button"
        className="flex h-9 shrink-0 items-center gap-1.5 px-3 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground"
        onClick={() => {
          if (expanded) {
            setExpanded(false);
            setPhase('closed');
          } else {
            setExpanded(true);
            restart();
          }
        }}
      >
        {expanded ? <ChevronDown className="size-3.5" /> : <ChevronUp className="size-3.5" />}
        <TerminalSquare className="size-3.5" />
        终端
        <span className="ml-1 text-[10px]">
          {phase === 'connecting' && '连接中…'}
          {phase === 'open' && '已连接'}
          {phase === 'exited' && exitCode !== null && `已退出（${exitCode}）`}
          {phase === 'error' && '连接失败'}
        </span>
      </button>
      {expanded ? (
        <div className="relative min-h-0 flex-1 bg-black px-2 pb-2">
          <div ref={termRef} className="h-full" />
          {(phase === 'exited' || phase === 'error') && (
            <div className="absolute inset-0 flex items-center justify-center gap-3 bg-black/70 text-xs text-muted-foreground">
              {phase === 'error' ? errorMsg : `会话已结束（退出码 ${exitCode ?? '?'}）`}
              <Button variant="outline" size="sm" onClick={restart}>
                <RotateCw className="mr-1 size-3" />
                重新开始
              </Button>
            </div>
          )}
        </div>
      ) : null}
    </div>
  );
}
