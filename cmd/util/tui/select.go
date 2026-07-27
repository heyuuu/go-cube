package tui

import (
	"charm.land/huh/v2"

	"github.com/heyuuu/cube/util/slicekit"
)

// Option 描述一个可选项：显示给用户的 Label 与返回给程序的 Value。
//
// 用于 Select 与 MultiSelect。Value 是泛型，可以是任意 comparable 类型
// （huh 要求泛型可比较，便于内部判等）。
type Option[T any] struct {
	Label string
	Value T
}

// Select 展示单选列表，返回被选中的值。
//
// 选项数小于等于 0 时返回 ErrEmptyOptions。
// 用户取消（Ctrl+C）时返回 ErrUserAborted。
// huh 的 Select 默认支持按 / 进入过滤模式，选项较多时可直接输入关键字筛选。
func Select[T any](title string, options []Option[T]) (T, error) {
	var zero T
	if err := mustTTY(); err != nil {
		return zero, err
	}
	if len(options) == 0 {
		return zero, ErrEmptyOptions
	}

	// init huh.Options
	huhOptions := make([]huh.Option[int], len(options))
	for i, o := range options {
		huhOptions[i] = huh.NewOption(o.Label, i)
	}

	// 启动 Select，返回选择 index
	var index int
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().Title(title).Options(huhOptions...).Value(&index),
		),
	).Run()
	if err != nil {
		return zero, normalizeError(err)
	}
	return options[index].Value, nil
}

// SelectItem 是 Select 的便利方法：把任意结构体切片映射成选项（用 labelGetter 提取展示名），返回被选中的原始项。
//
// 与 Select 的错误语义一致：用户取消返回 (零值, ErrUserAborted)。
func SelectItem[T any](title string, items []T, labelGetter func(T) string) (T, error) {
	return Select(title, slicekit.Map(items, func(item T) Option[T] {
		return Option[T]{Label: labelGetter(item), Value: item}
	}))
}

// MultiSelect 展示多选列表，返回所有被选中的值。
//
// 选项数小于等于 0 时返回 ErrEmptyOptions。
// 用户取消（Ctrl+C）时返回 ErrUserAborted。
// 用户一个都没选时返回长度为 0 的切片（非错误）。
// huh 的 MultiSelect 默认支持按 / 进入过滤模式。
func MultiSelect[T any](title string, options []Option[T]) ([]T, error) {
	if err := mustTTY(); err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, ErrEmptyOptions
	}

	// init huh.Options
	huhOptions := make([]huh.Option[int], len(options))
	for i, o := range options {
		huhOptions[i] = huh.NewOption(o.Label, i)
	}

	// 启动 Select，返回选择 indices
	var indices []int
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[int]().Title(title).Options(huhOptions...).Value(&indices),
		),
	).Run()
	if err != nil {
		return nil, normalizeError(err)
	}

	// 构建结果列表
	if len(indices) == 0 {
		return nil, nil
	}

	var results = make([]T, len(indices))
	for i, idx := range indices {
		results[i] = options[idx].Value
	}
	return results, nil
}

// MultiSelectItem 是 MultiSelect 的便利方法：把任意结构体切片映射成选项，返回
// 所有被选中的原始项（用索引中转，与 SelectItem 同理）。
func MultiSelectItem[T any](title string, items []T, labelGetter func(T) string) ([]T, error) {
	return MultiSelect(title, slicekit.Map(items, func(item T) Option[T] {
		return Option[T]{Label: labelGetter(item), Value: item}
	}))
}
