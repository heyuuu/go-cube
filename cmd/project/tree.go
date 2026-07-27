package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/heyuuu/cube/app"
	"github.com/heyuuu/cube/cmd/util/easycobra"
	"github.com/heyuuu/cube/cmd/util/tui"
	"github.com/heyuuu/cube/project"
	"github.com/heyuuu/cube/util/pathkit"
	"github.com/heyuuu/cube/util/slicekit"
)

var projectTreeCmd = &easycobra.Command{
	Use:   "tree",
	Short: "查看项目目录树",
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		// init flags
		var root string
		cmd.Flags().StringVar(&root, "root", "", "支持根目录")

		return func(args []string) error {
			// 项目列表
			service := app.Default().ProjectService()
			projects := service.Projects()
			if len(projects) == 0 {
				return fmt.Errorf("未找到任何项目，请确认配置是否正确")
			}

			// root 未指定时，默认取所有项目的最长公共前缀目录作为树根：
			// 多项目 → 共同祖先目录；单项目 → 项目自身。
			if root == "" {
				paths := make([]string, 0, len(projects))
				for _, p := range projects {
					paths = append(paths, filepath.Clean(p.Path()))
				}
				root = pathkit.CommonPrefix(paths)
			} else {
				// 指定的 root 规范化为绝对路径：
				// 先展开 ~，再 filepath.Abs（含 Clean，并支持相对路径如 .）
				root = pathkit.RealPath(root)
				if abs, err := filepath.Abs(root); err == nil {
					root = abs
				}
			}

			// 仅保留位于 root 子树下的项目
			projects = filterProjectsByRoot(projects, root)
			if len(projects) == 0 {
				return fmt.Errorf("没有任何符合条件的项目: root=%s", pathkit.PrettyPath(root))
			}

			// 渲染项目目录树
			treeRoot := buildProjectTree(projects, root)
			tui.PrintTree(treeRoot)

			return nil
		}
	},
}

// buildProjectTree 从一组项目构造以 root 为根的目录树。
//
// 规则：
//   - 根节点 = 调用方指定的 root，标签用 pathkit.PrettyPath 展示绝对路径；
//   - 仅展开「项目目录」或「包含项目目录的目录」；其余目录作为叶子节点显示但不再展开；
//   - 真实子目录通过 os.ReadDir 读取（忠实于磁盘）；
//   - 项目目录 Highlight=true（由 tui 层渲染为加粗高亮）；
//   - 同级节点按名称字典序排序。
func buildProjectTree(projects []*project.Project, root string) tui.TreeNode {
	// Project.Path() 已是绝对路径，这里仅做 Clean 保证后续前缀比较与磁盘读取一致。
	paths := make([]string, 0, len(projects))
	for _, p := range projects {
		paths = append(paths, filepath.Clean(p.Path()))
	}

	projectSet := slicekit.ToSet(paths)     // 项目目录集合（精确到项目路径本身）
	skeleton := buildProjectSkeleton(paths) // 含项目的目录集合（项目路径 + 所有祖先）
	// root 自身作为最顶层中转目录也需纳入 skeleton，保证其会被展开
	skeleton[root] = true

	// 根节点标签：PrettyPath 会把 home 目录下的绝对路径渲染成 ~ 形式
	rootLabel := pathkit.PrettyPath(root)

	return tui.TreeNode{
		Name:     rootLabel,
		Children: buildDirChildren(root, projectSet, skeleton),
	}
}

// filterProjectsByRoot 仅保留位于 root 子树下的项目（即路径等于 root 或以 root/ 为前缀）。
func filterProjectsByRoot(projects []*project.Project, root string) []*project.Project {
	root = filepath.Clean(root)
	result := make([]*project.Project, 0, len(projects))
	for _, p := range projects {
		path := filepath.Clean(p.Path())
		if path == root {
			result = append(result, p)
			continue
		}
		if strings.HasPrefix(path, root+string(filepath.Separator)) {
			result = append(result, p)
		}
	}
	return result
}

// buildDirChildren 读取 dir 的真实子目录，构造下一层节点。
// 仅当 dir 在 skeleton 中（即含项目）时才下钻；否则返回 nil（成为叶子）。
func buildDirChildren(dir string, projectSet, skeleton map[string]bool) []tui.TreeNode {
	if !skeleton[dir] {
		return nil // 当前目录不含项目：作为叶子停止展开
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // 不可读目录按叶子处理
	}

	// 收集子目录节点，并标记是否项目目录
	nodes := make([]tui.TreeNode, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// 跳过隐藏目录（与常见 tree 工具一致，也避免 .git 等噪声）
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		nodes = append(nodes, tui.TreeNode{
			Name:      e.Name(),
			Highlight: projectSet[filepath.Join(dir, e.Name())],
		})
	}

	// 同级排序：按名称字典序（本树只渲染目录节点，不存在目录/文件混排）
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })

	// 递归构造子节点：
	//   - 项目目录自身不再下钻（它是叶子终点）；
	//   - 非项目但「含项目」的目录（在 skeleton 中）才继续展开。
	for i, n := range nodes {
		if n.Highlight {
			continue
		}
		absPath := filepath.Join(dir, n.Name)
		if skeleton[absPath] {
			nodes[i].Children = buildDirChildren(absPath, projectSet, skeleton)
		}
	}
	return nodes
}

// buildProjectSkeleton 构造「含项目的目录集合」：
// 每条项目路径自身及其所有祖先目录都加入集合。
// 集合中的目录在树里要么是项目目录、要么是中转目录——都会被展开。
func buildProjectSkeleton(paths []string) map[string]bool {
	set := make(map[string]bool)
	for _, p := range paths {
		p = filepath.Clean(p)
		set[p] = true
		for dir := filepath.Dir(p); dir != p && dir != "" && dir != "."; dir = filepath.Dir(dir) {
			set[dir] = true
			if dir == string(filepath.Separator) {
				break
			}
		}
	}
	return set
}
