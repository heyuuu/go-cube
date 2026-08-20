package workbench

// PTY 会话（提案 1014）：一个前端页面对应一个会话，
// 连接建立时起子进程、断开即杀（不做重连保留，重连 = 新会话）。
// 协议为简单 JSON 帧：input/resize（入）、output/exit（出）。
// 会话注册表与停机广播在 Service（service.go 的 ServePty / StopPtySessions）。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/coder/websocket"
	"github.com/creack/pty"
)

// ptyMessage PTY WebSocket 帧结构。
type ptyMessage struct {
	Type string `json:"type"` // output / exit / input / resize
	Data string `json:"data"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
	Code int    `json:"code"`
}

// servePtySession 处理一个 PTY 会话的全部机制：起 shell → 双向泵 → 任一端结束后清理。
// 退出路径（三方任一结束都触发整体清理）：
//   - 客户端断开（read pump 出错）→ ctx cancel → 杀进程
//   - 子进程退出（ptmx Read io.EOF）→ 发 exit 帧 → 关连接
//   - server 关闭（外层 ctx cancel，由调用方传入）→ 杀进程
func servePtySession(ctx context.Context, conn *websocket.Conn, dir string, cols, rows int) error {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("path 目录不可用: path=%s: %w", dir, err)
	}

	cmd := exec.Command(shell)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return fmt.Errorf("启动 shell 失败: %w", err)
	}
	defer func() {
		// 兜底 SIGKILL：正常退出路径已 wait，这里只处理异常残留；不留孤儿进程是硬要求
		_ = ptmx.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	// 子进程退出信号（wait 只能调一次，由独立 goroutine 持有）
	procDone := make(chan error, 1)
	go func() { procDone <- cmd.Wait() }()

	// pty → 客户端 输出泵
	outputErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
				werr := conn.Write(wctx, websocket.MessageText,
					mustJSON(ptyMessage{Type: "output", Data: string(buf[:n])}))
				wcancel()
				if werr != nil {
					outputErr <- werr
					return
				}
			}
			if err != nil {
				outputErr <- err
				return
			}
		}
	}()

	// 客户端 → pty 输入泵（在调用方 goroutine 内同步读）
	readErr := make(chan error, 1)
	go func() {
		for {
			mt, data, err := conn.Read(ctx)
			if err != nil {
				readErr <- err
				return
			}
			if mt != websocket.MessageText {
				continue
			}
			var msg ptyMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "input":
				_, _ = ptmx.Write([]byte(msg.Data))
			case "resize":
				_ = pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(msg.Cols), Rows: uint16(msg.Rows)})
			}
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-readErr:
		// 客户端断开（含页面关闭）：杀进程退出
		slog.Debug("pty 客户端断开", "dir", dir, "err", err)
		return nil
	case err := <-outputErr:
		// pty 输出结束 = 子进程退出：发 exit 帧后收尾
		var exitCode int
		if waitErr := <-procDone; waitErr != nil {
			var exitErr *exec.ExitError
			if errors.As(waitErr, &exitErr) {
				exitCode = exitErr.ExitCode()
			}
		}
		wctx, wcancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = conn.Write(wctx, websocket.MessageText,
			mustJSON(ptyMessage{Type: "exit", Code: exitCode}))
		wcancel()
		_ = conn.Close(websocket.StatusNormalClosure, "process exited")
		_ = err
		return nil
	}
}

// mustJSON 序列化 PTY 帧，失败时兜底一个空 output 帧（不让单条编码失败中断泵）。
func mustJSON(msg ptyMessage) []byte {
	b, err := json.Marshal(msg)
	if err != nil {
		return []byte(`{"type":"output","data":""}`)
	}
	return b
}
