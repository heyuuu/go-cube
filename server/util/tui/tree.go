package tui

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
)

// TreeNodeStyle 枚举树节点的显示样式。
type TreeNodeStyle int

const (
	// TreeNodeStyleNone 默认样式（无特殊颜色）。
	TreeNodeStyleNone TreeNodeStyle = iota
	// TreeNodeStyleBlue 加粗 + 青色（color 51）。
	TreeNodeStyleBlue
	// TreeNodeStyleGreen 加粗 + 绿色。
	TreeNodeStyleGreen
)

// TreeNode 是一棵与业务无关的树形数据。
// 调用方按业务自行填充 Name / Style / Children，再交给 RenderTree / PrintTree 渲染。
//
//   - Name：节点显示文本；
//   - Style：节点样式（None 默认 / Blue 加粗青色 / Green 加粗绿色）；
//   - Children：子节点；为空则作为叶子节点。
type TreeNode struct {
	Name     string
	Style    TreeNodeStyle
	Children []TreeNode
}

// buildTree 把一棵 TreeNode 转成 lipgloss tree 用于渲染。
//
// 默认套用一组开箱即用的样式：
//   - Style=Blue 节点：加粗 + 青色（color 51）；
//   - Style=Green 节点：加粗 + 绿色（color 35）；
//   - Style=None 与根节点：默认样式。
//
// 内部通过自定义 tNode（实现 tree.Node）承载 Style 标记，
// 因为 lipgloss 的 ItemStyleFunc 只能通过 children.At(i) 反查节点属性。
func buildTree(root TreeNode) *tree.Tree {
	styles := map[TreeNodeStyle]lipgloss.Style{
		TreeNodeStyleBlue:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51")),
		TreeNodeStyleGreen: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("35")),
	}

	t := tree.New().
		Root(root.Name).
		ItemStyleFunc(func(children tree.Children, i int) lipgloss.Style {
			if n, ok := children.At(i).(*tNode); ok {
				if s, ok := styles[n.style]; ok {
					return s
				}
			}
			return lipgloss.NewStyle()
		})

	for _, child := range root.Children {
		t.Child(buildTNode(child))
	}
	return t
}

// buildTNode 递归把 TreeNode 转成 lipgloss tree.Node 实现。
func buildTNode(n TreeNode) *tNode {
	node := &tNode{name: n.Name, style: n.Style}
	for _, c := range n.Children {
		node.children = append(node.children, buildTNode(c))
	}
	return node
}

// RenderTree 把一棵 TreeNode 渲染成目录树字符串。
//
// 默认带样式：Style=Blue 加粗青色、Style=Green 加粗绿色，其余默认。
// 与 RenderTable 同属「环境无关的纯渲染」——返回的字符串包含未降级的 ANSI 转义码，
// 如需重定向到文件 / 管道或遵循 NO_COLOR，请改用 PrintTree。
func RenderTree(root TreeNode) string {
	t := buildTree(root)
	// tree.String() 不带尾换行，直接返回即可——换行是 Print* 的职责。
	return t.String()
}

// PrintTree 把一棵 TreeNode 渲染成目录树并打印到 stdout，末尾补一个换行。
//
// 等价于 Print(RenderTree(root) + "\n")：颜色降级由 Print 统一处理
// （非 TTY / NO_COLOR / 256 色等场景会自动剥离或量化），调用方无需关心。
func PrintTree(root TreeNode) {
	Print(RenderTree(root) + "\n")
}

// tNode 实现 tree.Node，额外承载 style 标记供 ItemStyleFunc 反查。
// children 非空 → 展开型节点；children 为空 → 叶子。
type tNode struct {
	name     string
	style    TreeNodeStyle
	children []*tNode
}

func (n *tNode) Value() string  { return n.name }
func (n *tNode) String() string { return n.name }
func (n *tNode) Children() tree.Children {
	// 必须返回类型化的 nil（NodeChildren(nil)），不能返回接口 nil——
	// lipgloss renderer 会直接调用 Children().Length()，接口 nil 会 panic。
	if len(n.children) == 0 {
		return tree.NodeChildren(nil)
	}
	nodes := make([]tree.Node, 0, len(n.children))
	for _, c := range n.children {
		nodes = append(nodes, c)
	}
	return tree.NodeChildren(nodes)
}
func (n *tNode) Hidden() bool   { return false }
func (n *tNode) SetHidden(bool) {}
func (n *tNode) SetValue(v any) {
	if s, ok := v.(string); ok {
		n.name = s
	}
}
