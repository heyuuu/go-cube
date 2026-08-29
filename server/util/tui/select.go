package tui

import (
	"charm.land/huh/v2"

	"cube/util/slicekit"
)

// Option 描述一个可选项：显示给用户的 Label 与返回给程序的 Value。
//
// 用于 Select 与 MultiSelect。Value 是泛型，可以是任意 comparable 类型
// （huh 要求泛型可比较，便于内部判等）。
type Option[T any] struct {
	Label string
	Value T
}

// maxSelectHeight 是 Select 列表的显示高度上限（行）：超过后列表内部滚动。
const maxSelectHeight = 15

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

	// 启动 Select，返回选择 index；显式限制列表高度（选项数 + 标题行，封顶
	// maxSelectHeight），否则选项很多时整个终端被列表占满，标题被顶出首屏
	var index int
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().Title(title).Height(min(len(huhOptions)+1, maxSelectHeight)).Options(huhOptions...).Value(&index),
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
	return MultiSelectWithDefaults(title, options, nil)
}

// MultiSelectWithDefaults 与 MultiSelect 一致，但可指定进入时默认勾选的值集合。
//
// selected 是「期望默认勾选」的值切片；函数会在 huh.Options 中预先勾选 label 匹配
// （按 Value 相等判断）的选项。selected 中的值若不在 options 内则忽略。
// 用户取消（Ctrl+C）时返回 ErrUserAborted；一个都没选时返回长度为 0 的切片。
func MultiSelectWithDefaults[T any](title string, options []Option[T], selected []T) ([]T, error) {
	if err := mustTTY(); err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, ErrEmptyOptions
	}

	// 计算默认勾选的 index 集合（huh.MultiSelect 的 Value 指针就是「已选集合」）
	// 把选中的值映射成 option index 列表，作为初始 Value 传入 huh。
	selectedSet := make(map[any]bool, len(selected))
	for _, v := range selected {
		selectedSet[v] = true
	}
	defaultIndices := make([]int, 0)
	for i, o := range options {
		if selectedSet[o.Value] {
			defaultIndices = append(defaultIndices, i)
		}
	}

	// init huh.Options
	huhOptions := make([]huh.Option[int], len(options))
	for i, o := range options {
		huhOptions[i] = huh.NewOption(o.Label, i)
	}

	// 显式设置字段高度（选项行数 + 标题 1 行）：huh v2.0.3 的 MultiSelect 在自动
	// 高度模式下会把标题行数从 viewport 高度里再扣一次（Select 无此问题），
	// 导致 N 个选项只显示 N-1 个、单选项时整个列表空白。
	indices := defaultIndices
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[int]().Title(title).Height(len(huhOptions) + 1).Options(huhOptions...).Value(&indices),
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
	return MultiSelectItemWithDefaults(title, items, labelGetter, nil)
}

// MultiSelectItemWithDefaults 是 MultiSelectItem 的带默认勾选版本：
// 传入默认勾选的 items 子集（需与 items 元素可比较），列表初始即勾选这些项。
//
// 适用于「默认全选 remotes」「默认勾选当前分支」等场景。
func MultiSelectItemWithDefaults[T any](
	title string,
	items []T,
	labelGetter func(T) string,
	defaults []T,
) ([]T, error) {
	return MultiSelectWithDefaults(title, slicekit.Map(items, func(item T) Option[T] {
		return Option[T]{Label: labelGetter(item), Value: item}
	}), defaults)
}
