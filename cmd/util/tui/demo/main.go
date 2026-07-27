// Package main 是 tui 包的演示程序。
//
// 运行：go run ./tui/demo
//
// 依次演示：Confirm / Select / SelectItem / MultiSelect / Input / PasswordInput /
// RenderTable。每个交互独立运行，用户可随时 Ctrl+C 中断（返回 ErrUserAborted）。
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/heyuuu/cube/cmd/util/tui"
)

// Fruit 用于演示 SelectItem：从名称里选，拿回完整结构体。
type Fruit struct {
	Name  string
	Price int // 单位：分
}

func main() {
	fmt.Println("=== tui 演示（charm.land 系列：huh + lipgloss）===")
	fmt.Println()

	// 1) Confirm
	if ok, err := tui.Confirm("是否开始演示？"); err != nil {
		abort(err)
	} else if !ok {
		fmt.Println("已取消。")
		return
	}

	// 2) Select：返回泛型值
	lang, err := tui.Select("请选择一种语言", []tui.Option[string]{
		{Label: "Go", Value: "go"},
		{Label: "Rust", Value: "rust"},
		{Label: "Python", Value: "python"},
	})
	if err != nil {
		abort(err)
	}
	fmt.Printf("→ 你选择了 %s\n\n", lang)

	// 3) SelectItem：返回原始结构体
	fruits := []Fruit{
		{Name: "苹果", Price: 500},
		{Name: "香蕉", Price: 300},
		{Name: "樱桃", Price: 800},
	}
	picked, err := tui.SelectItem("请选择一种水果", fruits, func(f Fruit) string {
		return fmt.Sprintf("%s（%d 分）", f.Name, f.Price)
	})
	if err != nil {
		abort(err)
	}
	fmt.Printf("→ 你选择了 %s\n\n", picked.Name)

	// 4) MultiSelect：返回多个值
	tags, err := tui.MultiSelect("请选择你感兴趣的标签（可多选）", []tui.Option[string]{
		{Label: "后端", Value: "backend"},
		{Label: "前端", Value: "frontend"},
		{Label: "运维", Value: "devops"},
		{Label: "数据", Value: "data"},
	})
	if err != nil {
		abort(err)
	}
	fmt.Printf("→ 你选择了：%s\n\n", strings.Join(tags, ", "))

	// 5) Input：带默认值、占位符、校验
	name, err := tui.Input(
		"请输入你的名字",
		"匿名用户",  // 默认值
		"例如：张三", // 占位符
		func(s string) error {
			if strings.TrimSpace(s) == "" {
				return errors.New("名字不能为空")
			}
			return nil
		},
	)
	if err != nil {
		abort(err)
	}
	fmt.Printf("→ 你好，%s\n\n", name)

	// 6) PasswordInput：掩码输入
	pwd, err := tui.PasswordInput("请输入密码（至少 4 位）", func(s string) error {
		if len(s) < 4 {
			return errors.New("密码至少 4 位")
		}
		return nil
	})
	if err != nil {
		abort(err)
	}
	fmt.Printf("→ 密码已接收（长度 %d），不会回显明文\n\n", len(pwd))

	// 7) RenderTable：返回字符串，由调用方输出（本包不接管 stdout）
	out := tui.RenderTable(
		[]string{"语言", "诞生年份", "主要范式"},
		[][]string{
			{"Go", "2009", "并发、简洁"},
			{"Rust", "2010", "内存安全、零成本抽象"},
			{"Python", "1991", "动态、多范式"},
		},
	)
	fmt.Println(out)
}

// abort 统一处理交互错误：用户取消则友好提示退出，其它错误打印后非零退出。
func abort(err error) {
	if errors.Is(err, tui.ErrUserAborted) {
		fmt.Println("\n已取消。")
		return
	}
	fmt.Fprintln(os.Stderr, "交互出错:", err)
	os.Exit(1)
}
