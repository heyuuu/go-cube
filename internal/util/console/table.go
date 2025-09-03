package console

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
	"strings"
)

func PrintTableFunc[T any](records []T, headers []string, rowGetter func(T) []string) {
	rows := slicekit.Map(records, rowGetter)
	PrintTable(headers, rows)
}

func PrintTable(headers []string, rows [][]string) {
	// 计算列数
	columnCount := len(headers)
	for _, line := range rows {
		if columnCount < len(line) {
			columnCount = len(line)
		}
	}

	// 计算每列宽度
	maxLen := make([]int, columnCount)
	for index, field := range headers {
		fieldWidth := unicodeWidth(field)
		if maxLen[index] < fieldWidth {
			maxLen[index] = fieldWidth
		}
	}
	for _, line := range rows {
		for index, field := range line {
			fieldWidth := unicodeWidth(field)
			if maxLen[index] < fieldWidth {
				maxLen[index] = fieldWidth
			}
		}
	}

	// 计算分隔线
	splitLineBuilder := strings.Builder{}
	splitLineBuilder.WriteString("+")
	for _, fieldLen := range maxLen {
		splitLineBuilder.WriteString(strings.Repeat("-", fieldLen+2))
		splitLineBuilder.WriteString("+")
	}
	splitLine := splitLineBuilder.String()

	// 绘制表格
	fmt.Println(splitLine)
	printTableLine(columnCount, headers, maxLen)
	fmt.Println(splitLine)
	for _, line := range rows {
		printTableLine(columnCount, line, maxLen)
	}
	fmt.Println(splitLine)
}

func printTableLine(columnCount int, fields []string, maxLen []int) {
	builder := strings.Builder{}
	builder.WriteString("|")
	for i := 0; i < columnCount; i++ {
		field := fields[i]
		fieldWidth := unicodeWidth(field)
		if fieldWidth < maxLen[i] {
			field = field + strings.Repeat(" ", maxLen[i]-fieldWidth)
		}
		builder.WriteString(" " + field + " |")
	}
	fmt.Println(builder.String())
}

// 计算字符串宽度，支持 unicode
func unicodeWidth(str string) int {
	var width int
	for _, r := range []rune(str) {
		rint := int64(r)
		if rint <= 0x0019 {
			width += 0
		} else if rint <= 0x1fff {
			width += 1
		} else if rint <= 0xff60 {
			width += 2
		} else if rint <= 0xff9f {
			width += 1
		} else {
			width += 2
		}
	}
	return width
}
