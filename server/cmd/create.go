package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"cube/app"
)

// cmd `cube create`（模板引擎：本地目录 / git 仓库，单模板或模板集）
//
// 两种主用法，其余是边缘校验：
//  1. cube create <目标路径> —— 交互式逐步创建（来源/模板名/变量缺啥问啥）；
//  2. cube create <目标路径> --tpl ... --tpl-name ... --var k=v ... —— 非交互一步生成。
func newCreateCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <目标路径> [--tpl 模板来源] [--tpl-name 模板名] [--var key=value ...]",
		Short: "使用模板生成项目（本地目录或 git 仓库）",
		Long: `使用模板生成项目（引擎是机制，模板是数据，协议见 template.yaml）。

目标路径必传，不存在时自动创建（含多级）。

--tpl 模板来源：本地目录或 git 仓库 url（--depth 1 clone 到临时目录）。
缺省时弹交互输入框，预填 config.json 的 create.templateSource。
来源根目录有 template.yaml 则为单模板；一级子目录各有则为模板集。

--tpl-name 模板名：模板集选择子模板用。单模板传名报错；
模板集指定了不存在的名报错（列出可用）；模板集缺省则交互选择。

--var key=value：模板变量，可多次。传未声明的变量报错，缺的交互提问。

示例：
  cube create my-app
  cube create my-app --tpl-name go-service --var author=heyu
  cube create my-app --tpl ~/templates --tpl-name full
  cube create my-app --tpl https://github.com/xxx/templates.git --tpl-name go-service --var author=heyu`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliVars, err := cmd.Flags().GetStringSlice("var")
			if err != nil {
				return err
			}
			vars, err := parseCliVars(cliVars)
			if err != nil {
				return err
			}
			tpl, _ := cmd.Flags().GetString("tpl")
			tplName, _ := cmd.Flags().GetString("tpl-name")
			return a.CreateService().Create(tpl, tplName, args[0], vars)
		},
	}

	cmd.Flags().String("tpl", "", "模板来源（本地目录或 git url），缺省交互输入")
	cmd.Flags().String("tpl-name", "", "模板集内的模板名，缺省交互选择")
	cmd.Flags().StringSlice("var", nil, "模板变量 key=value（可多次）")
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
