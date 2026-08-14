package project

import (
	"path"
	"testing"

	"cube/internal/testfixture"
)

// TestBuildTree_RootResolve 验证 BuildTree 的 root 解析：绝对路径/~ 前缀生效，相对路径报错
// （root 来自 web 请求，server 进程的 cwd 对其无意义）。
func TestBuildTree_RootResolve(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	repo := ws.MakeProjectDir(path.Join("root", "proj"))

	s := newServiceAt(t, root, "g1", 5)

	// 绝对路径 root：正常构建，且树上能找到项目
	tree, err := s.BuildTree(repo)
	if err != nil {
		t.Fatalf("BuildTree(绝对路径) 不应报错: %v", err)
	}
	if !containsTreeNodePath(tree, repo) {
		t.Fatalf("树上应包含项目 %s，实际 root=%s", repo, tree.Path)
	}

	// 相对路径 root：报错而非静默降级
	if _, err := s.BuildTree("./somewhere"); err == nil {
		t.Fatal("BuildTree(相对路径) 应报错")
	}
}

// containsTreeNodePath 深度优先查找树上是否存在指定 Path 的节点。
func containsTreeNodePath(n TreeNode, p string) bool {
	if n.Path == p {
		return true
	}
	for _, c := range n.Children {
		if containsTreeNodePath(c, p) {
			return true
		}
	}
	return false
}
