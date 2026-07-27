package tui

import (
	"os"

	"golang.org/x/term"
)

// IsTTY 报告 stdin 是否为一个交互式终端（TTY）。
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// mustTTY 校验 stdin 是否为交互式终端；非 TTY 时返回 ErrNotTTY。
//
// 所有交互类函数（Select / MultiSelect / Confirm / Input / PasswordInput）
// 在调用 huh 之前先过这一道关。渲染类函数（RenderTable）无需校验。
//
// 校验用 stdin 的 fd：term.IsTerminal(int(os.Stdin.Fd()))。
func mustTTY() error {
	if !IsTTY() {
		return ErrNotTTY
	}
	return nil
}
