package tui

import (
	"errors"

	"charm.land/huh/v2"
)

// ErrNotTTY 在 stdin 不是交互式终端（TTY）时，由交互类函数返回。
//
// 触发场景：管道输入（echo x | prog）、输入重定向（prog < file）、
// CI 环境、IDE 运行控制台等。
//
// 本包的设计原则是「绝不静默降级」：交互类函数（Select/Input 等）
// 在非 TTY 下不尝试用 accessible 模式自动作答，而是直接返回 ErrNotTTY，
// 由调用方决定降级策略（退到命令行 flag、用默认值、或直接退出）。
// 这避免了 CI/脚本里「无人输入却静默选了第一项」的隐患。
var ErrNotTTY = errors.New("tui: stdin is not a tty (interactive functions require a real terminal)")

// ErrUserAborted 表示用户取消了交互（按 Ctrl+C）。
//
// 由 Select / MultiSelect / Confirm / Input / PasswordInput 在用户中断时返回，
// 调用方可据此区分「取消」与「真实错误」。
var ErrUserAborted = errors.New("tui: user aborted")

// ErrEmptyOptions 在选项列表为空时由 Select / MultiSelect 返回。
var ErrEmptyOptions = errors.New("tui: options must not be empty")

// normalizeError 把 huh 返回的底层错误归一化为本包的对外错误。
//
// huh 在用户中断（Ctrl+C / Esc）时返回 huh.ErrUserAborted，这里原样透传，
// 让调用方可以 errors.Is(err, tui.ErrUserAborted) 判定「用户主动取消」。
// 其它错误（如非 TTY）原样返回。
func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrUserAborted
	}
	return err
}
