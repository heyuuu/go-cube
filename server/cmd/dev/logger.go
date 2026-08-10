package dev

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"cube/app"
)

// cmd `cube dev logger`
//
// 逐一打印 slog 的各类输出，便于开发期观察：
//   - 不同级别（Debug / Info / Warn / Error）的输出与过滤；
//   - 结构化字段（字符串、数值、布尔、错误、分组、LogValuer）；
//   - With / WithGroup 带来的属性继承。
func newLoggerCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logger",
		Short: "测试 slog 的各种输出（handler / 级别 / 结构化字段）",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLoggerDemo()
		},
	}
	return cmd
}

// loggerCase 描述一个可单独运行的 slog 用例：标题 + 执行函数。
// 执行函数接收一个已配置好的 *slog.Logger，自行决定如何打印。
type loggerCase struct {
	title string
	run   func() error
}

// loggerCases 是全部用例（顺序即展示顺序）。新增用例时往这里追加即可。
var loggerCases = []loggerCase{
	{"各级别", caseLevels},
	{"结构化字段", caseAttrs},
	{"错误属性", caseErrorAttr},
	{"分组属性", caseGroup},
	{"LogValuer（自定义类型）", caseLogValuer},
	{"多行 With 拼接", caseWith},
	{"WithGroup 嵌套", caseWithGroup},
}

func runLoggerDemo() error {
	cases := loggerCases
	for _, c := range cases {
		fmt.Printf("---------- %s ----------\n", c.title)
		if err := c.run(); err != nil {
			return err
		}
		fmt.Println()
	}
	return nil
}

// -------- 用例 --------

func caseLevels() error {
	// 四个级别各打一条，可结合 --level 观察被过滤的条目
	slog.Debug("调试信息", "count", 3)
	slog.Info("普通信息", "user", "heyu")
	slog.Warn("警告信息", "retry", 2)
	slog.Error("错误信息", "code", 500)
	return nil
}

func caseAttrs() error {
	// 各类常见结构化字段类型
	slog.Info("属性示例",
		"string", "hello",
		"int", 42,
		"float", 3.14,
		"bool", true,
		"slice", []string{"a", "b", "c"},
		"map", map[string]any{"k": "v"},
	)
	return nil
}

func caseErrorAttr() error {
	// Any 会把 error 当作 slog.LogValuer，输出 error 字符串
	err := errors.New("db connection refused")
	slog.Error("查询失败", "err", err, "query", "SELECT 1")

	// 包装后的 error 链
	wrapped := fmt.Errorf("调用失败: %w", err)
	slog.Error("再次包装", "err", wrapped)
	return nil
}

func caseGroup() error {
	// slog.Group 把多个字段归到同一命名空间下
	slog.Info("请求信息",
		slog.Group("request",
			"method", "GET",
			"path", "/api/users",
		),
		slog.Group("response",
			"status", 200,
			"ms", 12,
		),
	)
	return nil
}

// requestLog 是一个实现了 slog.LogValuer 的自定义类型，
// 展示如何让自定义结构以受控的字段集合输出。
type requestLog struct {
	Method string
	Path   string
	Status int
}

func (r requestLog) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("method", r.Method),
		slog.String("path", r.Path),
		slog.Int("status", r.Status),
	)
}

func caseLogValuer() error {
	req := requestLog{Method: "POST", Path: "/api/login", Status: 401}
	slog.Info("登录请求", "req", req)
	return nil
}

func caseWith() error {
	// With 返回一个带固定属性的新 logger，后续每条都带这些属性
	logger := slog.Default().With("service", "cube", "version", "1.0.0")
	logger.Info("启动完成")
	logger.Info("处理请求", "id", "req-1")
	logger.Error("处理失败", "id", "req-2", "err", "timeout")
	return nil
}

func caseWithGroup() error {
	// WithGroup 把后续所有属性归入一个命名空间，可嵌套
	logger := slog.Default().WithGroup("app").WithGroup("request")
	logger.Info("嵌套分组",
		"method", "GET",
		"path", "/api/projects",
		"ms", 8,
	)
	return nil
}
