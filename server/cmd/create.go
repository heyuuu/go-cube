package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"cube/app"
)

// cmd `cube create`（模板引擎：本地目录 / git 仓库，单模板或模板集）
func newCreateCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <模板来源> [模板名] <目标路径> [--var key=value ...]",
		Short: "使用模板生成项目（本地目录或 git 仓库）",
		Long: `使用模板生成项目（引擎是机制，模板是数据，协议见 template.yaml）。

模板来源为本地目录或 git 仓库（--depth 1 clone 到临时目录）。
来源根目录有 template.yaml 则为单模板；一级子目录各有 template.yaml 则为模板集，
模板集须用「模板名」参数指定子模板（缺省时交互式选择）。
目标路径已存在时必须是空目录。变量优先取 --var key=value，缺的交互式提问补齐。

示例：
  cube create ~/templates/go-service my-app
  cube create ~/templates go-service-full my-app
  cube create https://github.com/xxx/templates go-service my-app --var author=heyu`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliVars, err := cmd.Flags().GetStringSlice("var")
			if err != nil {
				return err
			}
			vars, err := parseCliVars(cliVars)
			if err != nil {
				return err
			}
			source, templateName, target := args[0], "", args[1]
			if len(args) == 3 {
				templateName, target = args[1], args[2]
			}
			return a.CreateService().Create(source, templateName, target, vars)
		},
	}

	cmd.Flags().StringSlice("var", nil, "模板变量，格式 --var key=value（可多次）")
	return cmd
}

// parseCliVars 解析 --var key=value 为 map，非法格式报错。
func parseCliVars(items []string) (map[string]string, error) {
	vars := make(map[string]string, len(items))
	for _, item := range items {
		key, value, ok := strings.Cut(item, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("--var 参数格式应为 key=value: %q", item)
		}
		if _, dup := vars[key]; dup {
			return nil, fmt.Errorf("--var 重复的变量: %s", key)
		}
		vars[key] = value
	}
	return vars, nil
}
