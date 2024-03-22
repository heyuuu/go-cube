package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"go-cube/internal/slicekit"
	"log"
	"strconv"
	"strings"
)

type cmdOpts[T any] struct {
	Root    *cobra.Command
	Use     string
	Short   string
	Aliases []string
	Args    cobra.PositionalArgs
	Init    func(cmd *cobra.Command, flags *T)
	Run     func(cmd *cobra.Command, flags *T, args []string)
}

func initCmd[T any](opts cmdOpts[T]) *cobra.Command {
	var flags T

	// cmd
	cmd := &cobra.Command{
		Use:     opts.Use,
		Short:   opts.Short,
		Aliases: opts.Aliases,
		Args:    opts.Args,
	}
	if opts.Run != nil {
		cmd.Run = func(cmd *cobra.Command, args []string) {
			opts.Run(cmd, &flags, args)
		}
	}

	// init
	if opts.Root != nil {
		opts.Root.AddCommand(cmd)
	} else {
		rootCmd.AddCommand(cmd)
	}
	if opts.Init != nil {
		opts.Init(cmd, &flags)
	}

	return cmd
}

func printTableFunc[T any](items []T, headers []string, valueGetters ...func(T) string) {
	body := slicekit.Map(items, func(item T) []string {
		line := make([]string, len(valueGetters))
		for i, getter := range valueGetters {
			line[i] = getter(item)
		}
		return line
	})
	printTable(headers, body)
}

func printTable(headers []string, body [][]string) {
	// 计算列数
	columnCount := len(headers)
	for _, line := range body {
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
	for _, line := range body {
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
	for _, line := range body {
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

func alfredSearchResult(items []AlfredListItem) {
	result := H{
		"items": items,
	}

	bytes, err := json.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(bytes))
}

func alfredSearchResultFunc[T any](items []T, title func(T) string, subTitle func(T) string, arg func(T) string) {
	listItems := slicekit.Map(items, func(item T) AlfredListItem {
		return AlfredListItem{
			Title:    title(item),
			SubTitle: subTitle(item),
			Arg:      arg(item),
		}
	})
	alfredSearchResult(listItems)
}

func strRightPad(s string, minLen int) string {
	if len(s) >= minLen {
		return s
	} else {
		return s + strings.Repeat(" ", minLen-len(s))
	}
}

func choiceItem[T any](title string, items []T, nameFn func(T) string) (item T, ok bool) {
	names := slicekit.Map(items, nameFn)
	idx := choiceItemIndex(title, names)
	if idx < 0 {
		return
	}
	return items[idx], true
}

func choiceItemIndex(title string, items []string) int {
	if len(items) == 0 {
		return -1
	}
	defaultIdx := 0
	for {
		// show items
		fmt.Printf("%s [%s]\n", title, items[0])
		for i, name := range items {
			fmt.Printf("  [%d] %s\n", i, name)
		}
		fmt.Print("> ")

		// read input
		var input string
		n, err := fmt.Scanln(&input)
		if err != nil || n == 0 { // 未输入或读取出错使用默认值
			return defaultIdx
		}

		inputNum, err := strconv.Atoi(input)
		if err == nil && 0 <= inputNum && inputNum < len(items) { // 输入数字且为合法索引则返回
			return inputNum
		}
	}
}
