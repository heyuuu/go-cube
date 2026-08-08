package tui

import (
	"charm.land/huh/v2"
)

type InputOptions struct {
	Title        string
	DefaultValue string
	Placeholder  string
	Validate     func(string) error
	Prompt       string
	Inline       bool
	Password     bool
	CharLimit    int // <=0 表示不限制
}

func runInput(opts InputOptions) (string, error) {
	if err := mustTTY(); err != nil {
		return "", err
	}

	value := opts.DefaultValue
	field := huh.NewInput().
		Title(opts.Title).
		Value(&value)
	if opts.Placeholder != "" {
		field = field.Placeholder(opts.Placeholder)
	}
	if opts.Validate != nil {
		field = field.Validate(opts.Validate)
	}
	if opts.Inline {
		field = field.Inline(true)
	}
	if opts.CharLimit > 0 {
		field = field.CharLimit(opts.CharLimit)
	}

	if opts.Prompt != "" {
		field = field.Prompt(opts.Prompt)
	} else if opts.Inline {
		field = field.Prompt(": ") // inline 的默认 prompt 改为 ": "
	}

	if opts.Password {
		field = field.EchoMode(huh.EchoModePassword)
	}

	err := huh.NewForm(huh.NewGroup(field)).Run()
	if err != nil {
		return "", normalizeError(err)
	}
	return value, nil

}

// Input 读取一行文本输入。
//
//   - defaultVal 非空时作为预填默认值（用户直接回车即返回它）；
//   - placeholder 非空时在输入为空时显示提示文案（不会成为返回值）；
//   - validate 非 nil 时作为校验：返回 error 则表单拒绝提交并提示。
//
// 用户取消（Ctrl+C）时返回 ("", ErrUserAborted)。
func Input(title, defaultVal, placeholder string, validate func(string) error) (string, error) {
	return runInput(InputOptions{
		Title:        title,
		DefaultValue: defaultVal,
		Placeholder:  placeholder,
		Validate:     validate,
	})
}

// InputInline 与 Input 的区别：Title 与输入框在同一行（huh Inline 模式），
// 而非 Title 独占一行。更紧凑，适合短问题 + 短答案的场景。
// 此外输入框前的提示符由默认的 "> " 改为 ": "，配合单行布局更自然。
//
// 参数与错误语义与 Input 完全一致。
func InputInline(title, defaultVal, placeholder string, validate func(string) error) (string, error) {
	return runInput(InputOptions{
		Title:        title,
		DefaultValue: defaultVal,
		Placeholder:  placeholder,
		Validate:     validate,
		Inline:       true,
		Prompt:       ": ",
	})
}

// PasswordInput 读取密码：输入字符以掩码显示，不回显明文。
//
//   - validate 非 nil 时作为校验（如长度/复杂度检查）。
//
// 用户取消（Ctrl+C）时返回 ("", ErrUserAborted)。
//
// 注意：返回的是明文密码，调用方应尽快使用并避免日志记录。
func PasswordInput(title string, validate func(string) error) (string, error) {
	return runInput(InputOptions{
		Title:    title,
		Validate: validate,
		Password: true,
	})
}
