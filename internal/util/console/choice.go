package console

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
	"strconv"
)

func ChoiceItem[T any](title string, items []T, nameGetter func(T) string) (item T, ok bool) {
	choices := slicekit.Map(items, nameGetter)
	idx := Choice(title, choices)
	if idx < 0 {
		return
	}
	return items[idx], true
}

func Choice(title string, choices []string) int {
	if len(choices) == 0 {
		return -1
	}
	defaultIdx := 0
	for {
		// show choices
		fmt.Printf("%s [%s]\n", title, choices[0])
		for i, name := range choices {
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
		if err == nil && 0 <= inputNum && inputNum < len(choices) { // 输入数字且为合法索引则返回
			return inputNum
		}
	}
}
