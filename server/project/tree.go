package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cube/util/pathkit"
	"cube/util/slicekit"
)

// ErrNoProjects 树构建时无可用项目（未扫描到 / root 下无项目）。
var ErrNoProjects = errors.New("未找到任何项目")

// TreeNodeStyle 节点样式标记，与渲染层（tui/web）解耦。
type TreeNodeStyle int

const (
	TreeNodeStyleNone    TreeNodeStyle = iota // 默认（普通目录）
	TreeNodeStyleDir                          // 含项目的目录（中转目录）
	TreeNodeStyleProject                      // 项目目录
)

// TreeNode 是一棵与渲染无关的目录树节点。cmd/tui 与 web 各自转成自己的展示形态。
type TreeNode struct {
	Name     string // 节点显示名（根节点为 PrettyPath，其余为目录名）
	Path     string // 节点绝对路径（根节点为 root，供 web 定位/打开用）
	Style    TreeNodeStyle
	Children []TreeNode
}

// BuildTree 从当前 service 的项目出发，构造以 root 为根的目录树。
//
// 规则（与 cmd/project/tree.go 的 buildProjectTree 一致）：
//   - root 为空时，取所有项目的最长公共前缀目录作为树根；
//   - 仅展开「项目目录」或「含项目目录的目录」；其余目录作为叶子不再下钻；
//   - 真实子目录通过 os.ReadDir 读取（忠实于磁盘），跳过隐藏目录；
//   - 项目目录标记 Project，含项目但非项目的目录标记 Dir，其余 None；
//   - 同级按名称字典序排序。
func (s *Service) BuildTree(root string) (TreeNode, error) {
	projects := s.Projects()
	if len(projects) == 0 {
		return TreeNode{}, ErrNoProjects
	}

	paths := slicekit.Map(projects, (*Project).Path)

	if root == "" {
		root = pathkit.CommonPrefix(paths)
	} else {
		// root 来自 web 请求，server 进程的 cwd 对其无意义，只接受绝对路径/~ 前缀
		abs, err := pathkit.StaticAbsPath(root)
		if err != nil {
			return TreeNode{}, fmt.Errorf("解析树根路径失败: root=%s err=%w", root, err)
		}
		root = abs
	}

	// 仅保留 root 子树下的项目
	projects = slicekit.Filter(projects, func(p *Project) bool {
		return pathkit.HasPrefix(p.Path(), root)
	})
	if len(projects) == 0 {
		return TreeNode{}, ErrNoProjects
	}

	paths = slicekit.Map(projects, (*Project).Path)
	projectSet := slicekit.ToSet(paths)
	skeleton := buildSkeleton(paths)

	return TreeNode{
		Name:     pathkit.PrettyPath(root),
		Path:     root,
		Style:    TreeNodeStyleDir,
		Children: buildTreeChildren(root, projectSet, skeleton),
	}, nil
}

// buildSkeleton 构造「含项目的目录集合」：每条项目路径自身及其所有祖先目录。
// 集合中的目录在树里要么是项目目录、要么是中转目录，都会被展开。
func buildSkeleton(paths []string) map[string]bool {
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

// buildTreeChildren 读取 dir 的真实子目录构造下一层节点。
// 仅当 dir 在 skeleton 中（含项目）才下钻；否则返回 nil（叶子）。
func buildTreeChildren(dir string, projectSet, skeleton map[string]bool) []TreeNode {
	if !skeleton[dir] {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	nodes := make([]TreeNode, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		absPath := filepath.Join(dir, e.Name())
		switch {
		case projectSet[absPath]: // 项目节点（叶子）
			nodes = append(nodes, TreeNode{Name: e.Name(), Path: absPath, Style: TreeNodeStyleProject})
		case skeleton[absPath]: // 含项目的目录（递归）
			children := buildTreeChildren(absPath, projectSet, skeleton)
			nodes = append(nodes, TreeNode{
				Name: e.Name(), Path: absPath, Style: TreeNodeStyleDir, Children: children,
			})
		default: // 其他目录（叶子）
			nodes = append(nodes, TreeNode{Name: e.Name(), Path: absPath, Style: TreeNodeStyleNone})
		}
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
	return nodes
}
