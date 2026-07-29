package debug

import (
	"errors"
	"fmt"
	"strings"

	"github.com/heyuuu/cube/cmd/util/easycobra"
	"github.com/heyuuu/cube/cmd/util/tui"
)

// cmd `cube debug tui [component]`
//
// 逐个演示 tui 包的组件，便于开发期调整样式 / 新增封装时快速看效果。
//   - 不带参数：交互式选择「全部」或某个具体组件；
//   - 带参数：直接演示匹配名称的组件（匹配不上则报错）。
var tuiCmd = &easycobra.Command{
	Use:   "tui [component]",
	Short: "演示 tui 包各组件（交互 / 渲染 / 错误语义）",
	Run:   runTuiDemo,
}

// demo 描述一个可单独演示的组件：名字 + 执行函数。
// 每个函数内部自行处理交互取消（用户 Ctrl+C 时打印提示并返回 nil，不中断后续）。
type demo struct {
	name string
	run  func() error
}

// demos 是所有演示项的展平列表（顺序即展示顺序）。
// 新增组件演示时，往这里追加一项即可。
var demos = []demo{
	{"Table", demoTable},
	{"Tree", demoTree},
	{"Select", demoSelect},
	{"MultiSelect", demoMultiSelect},
	{"Input", demoInput},
	{"InputInline", demoInputInline},
	{"Confirm", demoConfirm},
	{"ConfirmInline", demoConfirmInline},
	{"错误语义", demoErrorSemantics},
}

// demoNames 返回所有组件名，逗号分隔，用于参数匹配失败时的提示。
func demoNames() string {
	names := make([]string, len(demos))
	for i, d := range demos {
		names[i] = d.name
	}
	return strings.Join(names, ", ")
}

func runTuiDemo(args []string) error {
	// 带参数：按名称直接匹配组件，匹配不上则报错
	if len(args) > 0 {
		name := args[0]
		for _, d := range demos {
			if d.name == name {
				fmt.Printf("\n========== %s ==========\n", d.name)
				if e := d.run(); e != nil {
					return e
				}
				fmt.Println("\n演示完成。")
				return nil
			}
		}
		return fmt.Errorf("未找到组件: %s（可用: %s）", name, demoNames())
	}

	// 无参数：交互式选择
	options := make([]tui.Option[int], 0, len(demos)+1)
	options = append(options, tui.Option[int]{Label: "全部", Value: -1})
	for i, d := range demos {
		options = append(options, tui.Option[int]{Label: d.name, Value: i})
	}

	picked, err := tui.Select("选择要演示的组件", options)
	if err != nil {
		return abortErr("选择演示项", err)
	}

	// -1 表示全部；否则只跑选中的那一个
	targets := demos
	if picked >= 0 {
		targets = demos[picked : picked+1]
	}

	for _, d := range targets {
		fmt.Printf("\n========== %s ==========\n", d.name)
		if e := d.run(); e != nil {
			return e
		}
	}

	fmt.Println("\n演示完成。")
	return nil
}

// -------- 渲染类 --------

func demoTable() error {
	tui.PrintTable(
		[]string{"项目", "Path", "分支"},
		[][]string{
			{"cube", "~/Code/cube", "master"},
			{"other", "~/Code/other", "develop"},
		},
	)
	return nil
}

func demoTree() error {
	tui.PrintTree(tui.TreeNode{
		Name: "cube",
		Children: []tui.TreeNode{
			{Name: "cmd", Style: tui.TreeNodeStyleBlue, Children: []tui.TreeNode{
				{Name: "debug"},
				{Name: "ugly"},
			}},
			{Name: "util", Style: tui.TreeNodeStyleGreen, Children: []tui.TreeNode{
				{Name: "tui"},
				{Name: "git"},
			}},
		},
	})
	return nil
}

// -------- 交互类 --------

func demoSelect() error {
	picked, err := tui.Select("选择一个分支", []tui.Option[string]{
		{Label: "master", Value: "master"},
		{Label: "develop", Value: "develop"},
		{Label: "feature/x", Value: "feature/x"},
	})
	if err != nil {
		return abortErr("Select", err)
	}
	fmt.Printf("→ 选中: %s\n", picked)
	return nil
}

func demoMultiSelect() error {
	picked, err := tui.MultiSelect("选择多个标签", []tui.Option[string]{
		{Label: "go", Value: "go"},
		{Label: "rust", Value: "rust"},
		{Label: "ts", Value: "ts"},
	})
	if err != nil {
		return abortErr("MultiSelect", err)
	}
	fmt.Printf("→ 选中: %v\n", picked)
	return nil
}

func demoInput() error {
	text, err := tui.Input("输入项目名", "cube", "<名称>", nil)
	if err != nil {
		return abortErr("Input", err)
	}
	fmt.Printf("→ 输入: %s\n", text)
	return nil
}

func demoInputInline() error {
	text, err := tui.InputInline("输入项目名", "cube", "<名称>", nil)
	if err != nil {
		return abortErr("InputInline", err)
	}
	fmt.Printf("→ 输入: %s\n", text)
	return nil
}

func demoConfirm() error {
	ok, err := tui.Confirm("使用按钮式 Confirm?")
	if err != nil {
		return abortErr("Confirm", err)
	}
	fmt.Printf("→ 结果: %v\n", ok)
	return nil
}

func demoConfirmInline() error {
	ok, err := tui.ConfirmInline("使用流式 ConfirmInline?")
	if err != nil {
		return abortErr("ConfirmInline", err)
	}
	fmt.Printf("→ 结果: %v\n", ok)
	return nil
}

// -------- 错误语义 --------

func demoErrorSemantics() error {
	fmt.Println("（每个交互 Ctrl+C 都会返回 ErrUserAborted；")
	fmt.Println("  非 TTY 环境——管道/重定向——会返回 ErrNotTTY。）")
	fmt.Println("演示：下面这一步直接 Ctrl+C 可观察 ErrUserAborted 的退出路径。")
	_, err := tui.ConfirmInline("按 Ctrl+C 测试取消语义（或按 n 正常结束）")
	if err != nil {
		return abortErr("取消语义测试", err)
	}
	return nil
}

// abortErr 把交互取消与真实错误区分开：
//   - ErrUserAborted：打印友好提示后返回 nil（不算命令失败）；
//   - 其它错误：原样向上抛。
func abortErr(step string, err error) error {
	if errors.Is(err, tui.ErrUserAborted) {
		fmt.Printf("\n[%s] 用户取消，停止演示。\n", step)
		return nil
	}
	if errors.Is(err, tui.ErrNotTTY) {
		fmt.Printf("\n[%s] 非 TTY 环境，跳过交互演示。\n", step)
		return nil
	}
	return fmt.Errorf("[%s] 出错: %w", step, err)
}
