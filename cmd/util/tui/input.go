package tui

import (
	"charm.land/huh/v2"
)

// Input 读取一行文本输入。
//
//   - defaultVal 非空时作为预填默认值（用户直接回车即返回它）；
//   - placeholder 非空时在输入为空时显示提示文案（不会成为返回值）；
//   - validate 非 nil 时作为校验：返回 error 则表单拒绝提交并提示。
//
// 用户取消（Ctrl+C）时返回 ("", ErrUserAborted)。
func Input(title, defaultVal, placeholder string, validate func(string) error) (string, error) {
	if err := mustTTY(); err != nil {
		return "", err
	}

	value := defaultVal
	field := huh.NewInput().
		Title(title).
		Value(&value)
	if placeholder != "" {
		field = field.Placeholder(placeholder)
	}
	if validate != nil {
		field = field.Validate(validate)
	}

	err := huh.NewForm(huh.NewGroup(field)).Run()
	if err != nil {
		return "", normalizeError(err)
	}
	return value, nil
}

// PasswordInput 读取密码：输入字符以掩码显示，不回显明文。
//
//   - validate 非 nil 时作为校验（如长度/复杂度检查）。
//
// 用户取消（Ctrl+C）时返回 ("", ErrUserAborted)。
//
// 注意：返回的是明文密码，调用方应尽快使用并避免日志记录。
func PasswordInput(title string, validate func(string) error) (string, error) {
	if err := mustTTY(); err != nil {
		return "", err
	}

	var value string
	field := huh.NewInput().
		Title(title).
		Value(&value).
		EchoMode(huh.EchoModePassword)
	if validate != nil {
		field = field.Validate(validate)
	}

	err := huh.NewForm(huh.NewGroup(field)).Run()
	if err != nil {
		return "", normalizeError(err)
	}
	return value, nil
}
