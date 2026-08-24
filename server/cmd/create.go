package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"cube/app"
)

// cmd `cube create`（模板引擎，第①步：本地目录单模板）
func newCreateCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <模板目录> <目标路径> [--key=value ...]",
		Short: "使用本地模板目录生成项目",
		Long: `使用模板生成项目（引擎是机制，模板是数据，协议见 template.yaml）。

模板目录根下须有 template.yaml；目标路径已存在时必须是空目录。
变量优先取 --key=value，缺的交互式提问补齐。`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliVars, err := cmd.Flags().GetStringSlice("var")
			if err != nil {
				return err
			}
			vars, err := parseCliVars(cliVars)
			if err != nil {
				return err
			}
			return a.CreateService().Create(args[0], args[1], vars)
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
