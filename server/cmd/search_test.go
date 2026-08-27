package cmd

import (
	"path"
	"testing"

	"cube/internal/testfixture"
	"cube/project"
	"cube/settings"
)

// newCmdServiceAt 为测试构造一个扫描指定根目录的 project.Service。
// （project 包有同类 helper 但包内私有；cmd 测试按同样方式就地构造，
// 不沉淀进 testfixture 以避免其反向依赖 project。）
func newCmdServiceAt(t *testing.T, scanRoot, group string, maxDepth int) *project.Service {
	t.Helper()
	ws := testfixture.NewWorkspace(t)
	settingsFile := ws.Join("settings.json")
	scanRules := []project.ScanRule{
		{Group: group, Path: scanRoot, MaxDepth: maxDepth},
	}
	if err := settings.SaveSection(settingsFile, "scanRules", scanRules); err != nil {
		t.Fatalf("写测试 settings.json 失败: %v", err)
	}
	return project.NewService(settingsFile, ws.Mkdir("cache"))
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

// TestPickProject_LocalMode 验证 --local（cubex 入口）模式：
//   - query 缺省以 cwd 为起点定位项目（子目录内向上标定项目根）；
//   - 非 local 模式下 query 空匹配多个项目，非 TTY 无法交互选择而报错（守住原有行为）；
//   - 显式 query 不受 --local 干预，仍按名称搜索。
func TestPickProject_LocalMode(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	repoA := ws.MakeProjectDir(path.Join("root", "projA"))
	repoB := ws.MakeProjectDir(path.Join("root", "projB"))
	s := newCmdServiceAt(t, root, "g1", 5)

	orig := localMode
	defer func() { localMode = orig }()

	// 非 local：query 空匹配两个项目，走交互选择，非 TTY 下报错
	localMode = false
	if _, err := pickProject(s, ""); err == nil {
		t.Fatalf("非 local 模式下 query 空应因多项匹配无法交互而报错")
	}

	// local：query 缺省以 cwd 定位，在 projA 子目录内向上命中唯一项目
	localMode = true
	t.Chdir(ws.Mkdir(path.Join("root", "projA", "sub")))
	proj, err := pickProject(s, "")
	if err != nil {
		t.Fatalf("local 模式下 pickProject(\"\") 出错: %v", err)
	}
	if proj.Path() != repoA {
		t.Fatalf("local 模式应命中 %s，实际 %s", repoA, proj.Path())
	}

	// local 下显式 query 不干预：仍按名称搜索命中 projB
	proj, err = pickProject(s, "projB")
	if err != nil {
		t.Fatalf("pickProject(\"projB\") 出错: %v", err)
	}
	if proj.Path() != repoB {
		t.Fatalf("显式 query 应命中 %s，实际 %s", repoB, proj.Path())
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
