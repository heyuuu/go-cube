package tui

import (
	"charm.land/huh/v2"
)

// Confirm 是 y/n 确认交互，返回用户的选择。
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
