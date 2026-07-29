package tui

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
)

// Confirm 是 y/n 确认交互，返回用户的选择。
//
// 基于 huh 的按钮式 Confirm 字段：渲染 [ Yes ] [ No ]，左右切换 + 回车确认。
// 适合作为表单字段；缺点是视觉偏「弹框」，与 Input/Select 的轻量风格不统一。
//
// 用户取消（Ctrl+C）时返回 (false, ErrUserAborted)。
func Confirm(title string) (bool, error) {
	if err := mustTTY(); err != nil {
		return false, err
	}
	var ok bool
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Value(&ok),
		),
	).Run()
	if err != nil {
		return false, normalizeError(err)
	}
	return ok, nil
}

// ConfirmInline 是流式 y/n 确认，基于 InputInline（单行、charm 默认样式）：
// 问题与 "(Y 是 / N 否)" 提示放在同一行 Title，输入 y/n 单字符 + 回车提交，
// Validate 拦住非法字符。
//
// 与 Confirm（按钮式）的区别：视觉是输入框而非 [ Yes ] [ No ] 按钮块，
// 与 Input/Select 的轻量风格更协调。
//
// 用户取消（Ctrl+C）时返回 (false, ErrUserAborted)。
func ConfirmInline(title string) (bool, error) {
	value, err := runInput(InputOptions{
		Title:     title + " (Y 是 / N 否)",
		Inline:    true,
		Prompt:    ": ",
		CharLimit: 1,
		Validate: func(s string) error {
			lc := strings.ToLower(s)
			if lc != "y" && lc != "n" {
				return fmt.Errorf("请输入 Y 或 N")
			}
			return nil
		},
	})
	if err != nil {
		return false, err
	}
	return strings.ToLower(value) == "y", nil
}
