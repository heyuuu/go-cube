package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/colorprofile"
)

// Print 把字符串写到 stdout，并按 stdout 的真实环境自动降级颜色。
//
// 它是本包所有「打印到终端」路径的唯一出口——PrintTable / PrintTree
// 都先渲染好字符串、补好尾换行，再交给 Print 输出。降级逻辑集中在这里，
// 避免在每个 Print* 里重复实现。降级规则：
//   - 重定向到文件 / 管道（非 TTY）：剥离所有 ANSI，输出纯文本；
//   - NO_COLOR=1 或 TERM=dumb：剥离颜色；
//   - 真 TTY 但仅支持 256 色：把 truecolor 量化为 256 色；
//   - 支持 truecolor 的 TTY：原样输出。
//
// 注意：Print 不补尾换行——换行是调用方（PrintTable / PrintTree）的职责，
// 这样它对任意字符串都行为一致，不偷偷吞或加字符。
func Print(s string) {
	// colorprofile.NewWriter 在 Write 时按 os.Stdout 探测档位并降级。
	w := colorprofile.NewWriter(os.Stdout, os.Environ())
	fmt.Fprint(w, s)
}
