package cmd

import (
	"path"
	"testing"

	"cube/config"
	"cube/internal/testfixture"
	"cube/project"
)

// newCmdServiceAt 为测试构造一个扫描指定根目录的 project.Service。
// （project 包有同类 helper 但包内私有；cmd 测试按同样方式就地构造，
// 不沉淀进 testfixture 以避免其反向依赖 project。）
func newCmdServiceAt(t *testing.T, scanRoot, group string, maxDepth int) *project.Service {
	t.Helper()
	ws := testfixture.NewWorkspace(t)
	cfg := config.ProjectConfig{
		Scan: []config.ScanRuleConfig{
			{Group: group, Path: scanRoot, MaxDepth: maxDepth},
		},
	}
	return project.NewService(cfg, ws.Mkdir("cache"))
}

// TestSearchProjects_PathQueryCwd 验证路径 query 的 cwd 解析发生在 cmd 层：
// query="." 基于当前目录解析后命中所属项目（复现 cube info . 的完整链路）。
// domain 的 SearchByPath 已不收相对路径，此测试守住出口层的解析职责不回退。
func TestSearchProjects_PathQueryCwd(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	repo := ws.MakeProjectDir(path.Join("root", "proj"))

	s := newCmdServiceAt(t, root, "g1", 5)

	// 项目根：query="." 命中
	t.Chdir(repo)
	projs, err := searchProjects(s, ".", true)
	if err != nil {
		t.Fatalf("searchProjects(\".\") 出错: %v", err)
	}
	if len(projs) != 1 || projs[0].Path() != repo {
		t.Fatalf("searchProjects(\".\") 应命中 %s，实际 %v", repo, projs)
	}

	// 项目子目录：query="." 向上标定命中
	sub := ws.Mkdir(path.Join("root", "proj", "sub"))
	t.Chdir(sub)
	projs, err = searchProjects(s, ".", true)
	if err != nil {
		t.Fatalf("searchProjects(\".\") 出错: %v", err)
	}
	if len(projs) != 1 || projs[0].Path() != repo {
		t.Fatalf("searchProjects(\".\") 在子目录应向上命中 %s，实际 %v", repo, projs)
	}
}

// TestSearchProjects_QueryModes name query 不走路径解析；不支持的路径语法显式报错。
func TestSearchProjects_QueryModes(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	ws.MakeProjectDir(path.Join("root", "proj"))

	s := newCmdServiceAt(t, root, "g1", 5)

	// name query：不走路径分支
	projs, err := searchProjects(s, "proj", false)
	if err != nil {
		t.Fatalf("searchProjects(name) 出错: %v", err)
	}
	if len(projs) != 1 {
		t.Fatalf("name 搜索应命中 1 个，实际 %d", len(projs))
	}

	// ~user 语法不支持：显式报错（而非静默空列表）
	if _, err = searchProjects(s, "~user/x", false); err == nil {
		t.Fatal("~user 语法应报错")
	}
}
