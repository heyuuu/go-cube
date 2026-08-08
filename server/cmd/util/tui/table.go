package tui

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// TableOption 配置 Table 的渲染样式。传入 nil 表示跳过该项。
//
// 之所以做成函数选项，是因为 lipgloss table 的边框/样式开关较多，
// 直接暴露 lipgloss 类型会泄漏实现依赖。
type TableOption func(*table.Table)

// applyDefaultStyle 给 table 套上一组开箱即用的好看样式：
//   - 圆角边框，边框着色（与表头呼应的紫色调）；
//   - 表头加粗 + 青色；
//   - 表头与首行数据之间画分隔线；
//   - 单元格左右留 1 格内边距。
//
// 这组样式与 console 包 huh 后端的 PrintTable 视觉一致，调用方零配置即得彩色表格。
// 后续的 TableOption 在此基础上增量覆盖（如 WithBorder(BorderNone) 可关掉边框）。
//
// 注意：表头与单元格必须使用相同的 Padding，否则表头文字会和下方单元格
// 在垂直方向上错位（lipgloss table 不自动对齐 padding）。这里两者都用 Padding(0,1)。
func applyDefaultStyle(t *table.Table) {
	padding := lipgloss.NewStyle().Padding(0, 1)
	headerStyle := padding.Bold(true).Foreground(lipgloss.Color("51"))
	cellStyle := padding

	t.Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("99"))).
		BorderHeader(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})
}

// buildTable 构造一个填好数据的 lipgloss table
// 默认套用 applyDefaultStyle（圆角彩色边框 + 加粗表头 + 内边距），opts 在此之上增量覆盖。行长度短于表头的列以空串补齐。
func buildTable(headers []string, rows [][]string, opts ...TableOption) *table.Table {
	t := table.New()
	applyDefaultStyle(t)
	for _, opt := range opts {
		if opt != nil {
			opt(t)
		}
	}
	if len(headers) > 0 {
		t.Headers(headers...)
	}
	// 规整每行列数到表头长度，避免参差不齐。
	col := len(headers)
	if col > 0 {
		normalized := make([][]string, len(rows))
		for i, r := range rows {
			row := make([]string, col)
			copy(row, r)
			normalized[i] = row
		}
		t.Rows(normalized...)
	} else if len(rows) > 0 {
		// 没有表头时，直接按原样渲染。
		t.Rows(rows...)
	}
	return t
}

// RenderTable 把 headers + rows 渲染成带对齐边框的表格字符串。
//
// 默认带样式（圆角彩色边框 + 加粗表头 + 内边距），与 console 包 huh 后端视觉一致。
// 如需调整，传入 TableOption 增量覆盖默认样式。
//
// 注意：返回的字符串包含未降级的 ANSI 转义码。如果要把结果重定向到文件、
// 管道，或希望遵循 NO_COLOR / TERM=dumb，请改用 PrintTable——它会按 stdout
// 的真实环境自动降级（剥离颜色或降到 256 色）。
func RenderTable(headers []string, rows [][]string, opts ...TableOption) string {
	t := buildTable(headers, rows, opts...)
	return t.Render()
}

// PrintTable 把 headers + rows 渲染成表格并打印到 stdout，末尾补一个换行。
//
// 等价于 Print(RenderTable(...) + "\n")：颜色降级由 Print 统一处理
// （非 TTY / NO_COLOR / 256 色等场景会自动剥离或量化），调用方无需关心。
//
// 适合绝大多数「我就是要打印这张表」的场景。需要拿字符串拼布局 / 做测试时，
// 用 RenderTable。
func PrintTable(headers []string, rows [][]string, opts ...TableOption) {
	Print(RenderTable(headers, rows, opts...) + "\n")
}
