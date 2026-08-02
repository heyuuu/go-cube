package project

import (
	"path"
	"testing"

	"github.com/heyuuu/cube/config"
	"github.com/heyuuu/cube/internal/testfixture"
)

// newServiceAt 为测试构造一个扫描指定根目录的 Service。
// 就地写而非放进 testfixture，是因为 testfixture 不能 import project（会导致 gogit 等底层包测试循环依赖）。
func newServiceAt(t *testing.T, scanRoot, group string, maxDepth int) *Service {
	t.Helper()
	ws := testfixture.NewWorkspace(t)
	conf := config.ProjectConfig{
		Scan: []config.ScanRuleConfig{
			{Group: group, Path: scanRoot, MaxDepth: maxDepth},
		},
	}
	return NewService(conf, ws.Mkdir("cache"))
}

// TestScan_SingleProject 单个 git 项目被识别。
func TestScan_SingleProject(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	repo := ws.MakeGitRepoWith(path.Join("root", "myproj"), testfixture.GitRepoSpec{})

	s := newServiceAt(t, root, "g1", 5)
	projs := s.Projects()

	if len(projs) != 1 {
		t.Fatalf("扫描出 %d 个 project，期望 1：%v", len(projs), projs)
	}
	if projs[0].Path() != repo {
		t.Fatalf("project path = %q，期望 %q", projs[0].Path(), repo)
	}
	if projs[0].Group() != "g1" {
		t.Fatalf("group = %q，期望 g1", projs[0].Group())
	}
}

// TestScan_NestedProjects 多个嵌套 git 项目都被识别。
func TestScan_NestedProjects(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	p1 := ws.MakeProjectDir(path.Join("root", "a"))
	p2 := ws.MakeProjectDir(path.Join("root", "b", "c"))

	s := newServiceAt(t, root, "g", 5)
	projs := s.Projects()

	if len(projs) != 2 {
		t.Fatalf("扫描出 %d 个，期望 2", len(projs))
	}
	paths := map[string]bool{}
	for _, p := range projs {
		paths[p.Path()] = true
	}
	if !paths[p1] || !paths[p2] {
		t.Fatalf("扫描结果不全：%v", paths)
	}
}

// TestScan_MaxDepth 深度剪枝：超 MaxDepth 的项目不被扫到。
func TestScan_MaxDepth(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	// 浅层项目（root/proj）
	ws.MakeProjectDir(path.Join("root", "proj"))
	// 深层项目（root/x/y/deep）—— root 到 deep 跨 3 层目录
	ws.MakeProjectDir(path.Join("root", "x", "y", "deep"))

	// MaxDepth=2：只扫到浅层
	s := newServiceAt(t, root, "g", 2)
	if len(s.Projects()) != 1 {
		t.Fatalf("MaxDepth=2 应只扫到 1 个，实际 %d", len(s.Projects()))
	}
}

// TestScan_NonProjectDirIgnored 不含 .git 的目录被忽略。
func TestScan_NonProjectDirIgnored(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	ws.Mkdir(path.Join("root", "plain-dir")) // 无 .git
	ws.MakeProjectDir(path.Join("root", "real-proj"))

	s := newServiceAt(t, root, "g", 5)
	if len(s.Projects()) != 1 {
		t.Fatalf("应只识别 git project，实际 %d", len(s.Projects()))
	}
}

// TestScan_DotUnderscoreDirSkipped 以 . 或 _ 开头的目录被跳过。
func TestScan_DotUnderscoreDirSkipped(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	// .git 命名的「伪 project」目录应被跳过（.git 本身也含 .git？不会，scan.go 直接 SkipDir）
	ws.Mkdir(path.Join("root", ".hidden"))
	ws.MakeProjectDir(path.Join("root", ".hidden", "inner")) // 在 .hidden 下，应被跳过
	ws.MakeProjectDir(path.Join("root", "normal"))

	s := newServiceAt(t, root, "g", 5)
	if len(s.Projects()) != 1 {
		t.Fatalf(". 开头目录应被跳过，实际扫到 %d 个", len(s.Projects()))
	}
}

// TestScan_GodotTag godot 项目打 godot tag。
func TestScan_GodotTag(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	ws.MakeProjectDir(path.Join("root", "game"), testfixture.WithGodot())

	s := newServiceAt(t, root, "g", 5)
	projs := s.Projects()
	if len(projs) != 1 {
		t.Fatalf("应扫到 1 个，实际 %d", len(projs))
	}
	tags := projs[0].Tags()
	found := false
	for _, tg := range tags {
		if tg == TagGodot {
			found = true
		}
	}
	if !found {
		t.Fatalf("godot 项目应打 godot tag，实际 tags=%v", tags)
	}
}

// TestScan_WorktreeTag .git 是文件的 worktree 打 worktree tag。
func TestScan_WorktreeTag(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	ws.MakeProjectDir(path.Join("root", "wt"), testfixture.WithWorktree())

	s := newServiceAt(t, root, "g", 5)
	projs := s.Projects()
	if len(projs) != 1 {
		t.Fatalf("应扫到 1 个，实际 %d", len(projs))
	}
	tags := projs[0].Tags()
	found := false
	for _, tg := range tags {
		if tg == TagWorktree {
			found = true
		}
	}
	if !found {
		t.Fatalf("worktree 应打 worktree tag，实际 tags=%v", tags)
	}
}

// TestFindByPathAndName FindByPath / FindByName 查找。
func TestFindByPathAndName(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	repo := ws.MakeProjectDir(path.Join("root", "proj"))

	s := newServiceAt(t, root, "g1", 5)

	if p := s.FindByPath(repo); p == nil || p.Path() != repo {
		t.Fatalf("FindByPath 未找到: %v", s.FindByPath(repo))
	}
	if p := s.FindByPath("/nonexistent"); p != nil {
		t.Fatalf("不存在 path 应返回 nil")
	}

	// name 格式 = group:subpath，这里 = g1:proj
	if p := s.FindByName("g1:proj"); p == nil {
		t.Fatalf("FindByName 未找到 g1:proj")
	}
}

// TestScanRules_Getter ScanRules() 返回构造时传入的规则。
func TestScanRules_Getter(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	conf := config.ProjectConfig{
		Scan: []config.ScanRuleConfig{{Group: "g", Path: ws.Mkdir("root"), MaxDepth: 3}},
	}
	s := NewService(conf, ws.Mkdir("cache"))
	rules := s.ScanRules()
	if len(rules) != 1 || rules[0].Group != "g" {
		t.Fatalf("ScanRules 异常: %v", rules)
	}
}
