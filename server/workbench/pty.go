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

// mustJSON 序列化 PTY 帧，失败时兜底一个空 output 帧（不让单条编码失败中断泵）。
func mustJSON(msg ptyMessage) []byte {
	b, err := json.Marshal(msg)
	if err != nil {
		return []byte(`{"type":"output","data":""}`)
	}
	return b
}
