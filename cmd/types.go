package cmd

import "go-cube/internal/slicekit"

type H map[string]any

type AlfredListItem struct {
	Title    string `json:"title"`
	SubTitle string `json:"subtitle"`
	Arg      string `json:"arg"`
}

// 表格打印工具
type TableDef[T any] struct {
	headers      []string
	valueGetters []func(T) string
}

func (t *TableDef[T]) AddCol(title string, valueGetter func(T) string) {
	t.headers = append(t.headers, title)
	t.valueGetters = append(t.valueGetters, valueGetter)
}

func (t *TableDef[T]) Print(items []T) {
	body := slicekit.Map(items, func(item T) []string {
		return slicekit.Map(t.valueGetters, func(getter func(T) string) string {
			return getter(item)
		})
	})
	printTable(t.headers, body)
}
