package workbench

import (
	"encoding/json"
)

// PTY 会话（提案 1014）：一个前端页面对应一个会话，
// 连接建立时起子进程、断开即杀（不做重连保留，重连 = 新会话）。
// 协议为简单 JSON 帧：input/resize（入）、output/exit（出）。

type ptyMessage struct {
	Type string `json:"type"` // input | resize | output | exit
	Data string `json:"data"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
	Code int    `json:"code"`
}

// ServePty 处理一个 PTY WebSocket 连接：升级 → 起子进程 → 双向泵 → 任一端结束后清理。
// 退出路径（三方任一结束都触发整体清理）：
//   - 客户端断开（read pump 出错）→ cancel → 杀进程
//   - 子进程退出（ptmx Read io.EOF）→ 发 exit 帧 → 关连接
//   - server 关闭（ctx cancel）→ 杀进程
func mustJSON(msg ptyMessage) []byte {
	b, err := json.Marshal(msg)
	if err != nil {
		return []byte(`{"type":"output","data":""}`)
	}
	return b
}

// StopPtySessions 向所有活跃 PTY 会话发取消（server 停机时调用），确保无孤儿 shell。
// MVP 用 Service 上的轻量注册表；会话数 = 浏览器页面数，量级极小。
