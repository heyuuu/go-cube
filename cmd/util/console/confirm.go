package console

import (
	"fmt"
	"strings"
)

// Confirm 是一个 y/n 交互式确认。默认按「是」处理（直接回车 = 是）。
//
// 与 Choice 使用同样的 fmt.Scanln 读取方式，避免与 Choice 混用时 stdin 缓冲冲突。
func Confirm(title string) bool {
	for {
		fmt.Printf("%s [Y/n] ", title)

		var input string
		n, err := fmt.Scanln(&input)
		if err != nil || n == 0 { // 未输入或读取出错，使用默认值「是」
			return true
		}

		switch strings.ToLower(strings.TrimSpace(input)) {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
		// 其它输入，重新询问
	}
}
