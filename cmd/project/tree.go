package project

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/heyuuu/cube/app"
	"github.com/heyuuu/cube/cmd/util/easycobra"
	"github.com/heyuuu/cube/cmd/util/tui"
	"github.com/heyuuu/cube/project"
)

var treeCmd = &easycobra.Command{
	Use:   "tree",
	Short: "查看项目目录树",
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		// init flags
		var root string
		cmd.Flags().StringVar(&root, "root", "", "支持根目录")

		return func(args []string) error {
			service := app.Default().ProjectService()

			// 树构建逻辑下沉在 project.Service.BuildTree（与 web 共用）
			treeRoot, err := service.BuildTree(root)
			if err != nil {
				if errors.Is(err, project.ErrNoProjects) {
					fmt.Println("未找到任何项目，请确认配置是否正确")
					return nil
				}
				return err
			}

			// 渲染：project.TreeNode → tui.TreeNode（领域模型 → 渲染模型）
			tui.PrintTree(toTuiNode(treeRoot))
			return nil
		}
	},
}

// toTuiNode 把领域层的 project.TreeNode 转成 tui.TreeNode（cmd 渲染专用）。
// 样式映射：Project → Green（加粗绿）；Dir → Blue（加粗青）；None → None。
func toTuiNode(n project.TreeNode) tui.TreeNode {
	var style tui.TreeNodeStyle
	switch n.Style {
	case project.TreeNodeStyleProject:
		style = tui.TreeNodeStyleGreen
	case project.TreeNodeStyleDir:
		style = tui.TreeNodeStyleBlue
	}
	children := make([]tui.TreeNode, 0, len(n.Children))
	for _, c := range n.Children {
		children = append(children, toTuiNode(c))
	}
	return tui.TreeNode{Name: n.Name, Style: style, Children: children}
}
