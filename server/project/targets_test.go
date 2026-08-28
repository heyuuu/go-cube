package project

import (
	"os"
	"path/filepath"
	"testing"

	"cube/internal/testfixture"
	"cube/project/projcache"
	"cube/project/workspace"
)

func entryFixture() *projcache.Entry {
	return &projcache.Entry{
		Workspaces: []workspace.Workspace{{Name: "server", Path: "server"}, {Name: "web", Path: "web"}},
		Worktrees: []projcache.WorktreeInfo{
			{Path: "/x/wt-hot", Branch: "hotfix"},
			{Path: "/x/wt-detached", Branch: "", Detached: true},
			{Path: "/x/wt-a", Branch: "feat"},
			{Path: "/x/wt-b", Branch: "feat"}, // 与 wt-a 同分支，双双回退目录名
		},
	}
}

// TestTargetEntries 展示名规则（分支名优先，detached/同分支冲突回退目录名）
// + 四段排序（主 workspaces → 各 worktree 根 → 其 workspaces）。
// workspace 成员目录须真实存在（存在性兜底），fixture 里在建仓目录下现建。
func TestTargetEntries(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Dir
	for _, sub := range []string{"server", "web"} {
		ws.Mkdir(sub)
	}

	info := entryFixture()
	info.Worktrees[0].Workspaces = []workspace.Workspace{{Name: "server", Path: "server"}}

	got := targetEntries(root, info)
	type wantItem struct {
		label string
		rel   string // 期望路径（空 = 由测试另行断言）
		flags TargetFlags
	}
	want := []wantItem{
		{"server", "server", FlagWorkspace},
		{"web", "web", FlagWorkspace},
		{"hotfix", "/x/wt-hot", FlagWorktree}, // worktree 根不可 stat，被存在性过滤
		{"wt-detached", "/x/wt-detached", FlagWorktree},
		{"wt-a", "/x/wt-a", FlagWorktree},
		{"wt-b", "/x/wt-b", FlagWorktree},
	}
	if len(got) != len(want) {
		t.Fatalf("应返回 %d 个目标, got %d (%+v)", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i].Label != w.label || got[i].Flags != w.flags {
			t.Errorf("第 %d 个不符: want %s/%v, got %s/%v", i, w.label, w.flags, got[i].Label, got[i].Flags)
		}
		if w.rel != "" && w.rel[0] == '/' && got[i].Path != w.rel {
			t.Errorf("第 %d 个路径不符: want %s, got %s", i, w.rel, got[i].Path)
		}
	}
	// workspace 目标是绝对路径且指向主根下
	if got[0].Path != filepath.Join(root, "server") || got[0].Branch != "" {
		t.Errorf("workspace 目标不符: %+v", got[0])
	}

	if targetEntries(root, nil) != nil {
		t.Error("nil entry 应返回 nil")
	}
}

// TestTargetEntriesWorktreeWorkspaces worktree 根下的 workspace 同样展开，标记为组合
// FlagWorktree|FlagWorkspace（交叉身位），排序紧跟其 worktree 根。
func TestTargetEntriesWorktreeWorkspaces(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	wtRoot := ws.Mkdir("wt")
	ws.Mkdir(filepath.Join("wt", "app"))

	got := targetEntries("/main", &projcache.Entry{
		Worktrees: []projcache.WorktreeInfo{{
			Path:       wtRoot,
			Branch:     "feat",
			Workspaces: []workspace.Workspace{{Name: "app", Path: "app"}},
		}},
	})
	if len(got) != 2 {
		t.Fatalf("应返回 worktree 根 + 1 个 workspace, got %+v", got)
	}
	if got[0].Flags != FlagWorktree || got[0].Label != "feat" {
		t.Errorf("worktree 根目标不符: %+v", got[0])
	}
	if got[1].Flags != FlagWorktree|FlagWorkspace || got[1].Path != filepath.Join(wtRoot, "app") || got[1].Label != "app" {
		t.Errorf("worktree 下 workspace 目标不符: %+v", got[1])
	}
}

// TestOpenTargets 根目录在前 + 快照内目标跟随；未采集/无附加目标项目只有根目录。
func TestOpenTargets(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	repo := ws.MakeProjectDir(filepath.Join("root", "repo"))

	s := newServiceWithRules(t, ws, []ScanRule{{Group: "g", Path: root, MaxDepth: 3}}, nil)

	// 未采集快照时只有根目录目标
	targets := s.OpenTargets(repo)
	if len(targets) != 1 || targets[0].Path != repo || targets[0].Label != "根目录" || targets[0].Flags != 0 {
		t.Fatalf("无快照应只有根目录目标: %+v", targets)
	}

	// 建 worktree + 声明 workspace，真实采集后目标列表 = 根目录 + workspace + worktree（四段排序）
	ws.MakeWorktree(repo, "wt-hot", "hotfix")
	if err := os.MkdirAll(filepath.Join(repo, ".cube"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "server"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".cube", "cube.json"),
		[]byte(`{"workspaces":[{"name":"服务端","path":"server"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Refresh(); err != nil {
		t.Fatalf("Refresh 失败: %v", err)
	}

	targets = s.OpenTargets(repo)
	if len(targets) != 3 {
		t.Fatalf("应有 3 个目标（根 + workspace + worktree), got %+v", targets)
	}
	if targets[0].Path != repo || targets[0].Flags != 0 {
		t.Fatalf("根目录目标不符: %+v", targets[0])
	}
	if targets[1].Flags != FlagWorkspace || targets[1].Path != filepath.Join(repo, "server") || targets[1].Label != "服务端" {
		t.Fatalf("workspace 目标不符: %+v", targets[1])
	}
	canonicalWt, err := filepath.EvalSymlinks(ws.Join("wt-hot"))
	if err != nil {
		t.Fatalf("解析真实路径失败: %v", err)
	}
	if targets[2].Path != canonicalWt || targets[2].Label != "hotfix" || targets[2].Branch != "hotfix" || targets[2].Flags != FlagWorktree {
		t.Fatalf("worktree 目标不符: %+v", targets[2])
	}

	// 未收录路径返回 nil
	if s.OpenTargets("/nonexistent") != nil {
		t.Fatal("未收录路径应返回 nil")
	}
}

// TestResolveMainProject worktree 目录（含子目录）归并到主项目；主仓库未收录/非 worktree 返回 nil。
func TestResolveMainProject(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	repo := ws.MakeProjectDir(filepath.Join("root", "repo"))
	wtDir := ws.MakeWorktree(repo, "wt-out", "feat") // worktree 位于 scan root 之外

	s := newServiceWithRules(t, ws, []ScanRule{{Group: "g", Path: root, MaxDepth: 3}}, nil)

	// worktree 根目录与其子目录都归并到主项目
	if p := s.ResolveMainProject(wtDir); p == nil || p.Path() != repo {
		t.Fatalf("worktree 应归并到主项目 %s, got %+v", repo, p)
	}
	sub := ws.Mkdir(filepath.Join("wt-out", "sub"))
	if p := s.ResolveMainProject(sub); p == nil || p.Path() != repo {
		t.Fatalf("worktree 子目录应归并到主项目 %s, got %+v", repo, p)
	}
	// 主仓库目录自身不是 worktree，返回 nil
	if p := s.ResolveMainProject(repo); p != nil {
		t.Fatalf("主仓库目录应返回 nil, got %+v", p)
	}
	// 普通目录返回 nil
	if p := s.ResolveMainProject(ws.Join("plain")); p != nil {
		t.Fatalf("普通目录应返回 nil, got %+v", p)
	}
}

// TestResolveProject 项目根直接命中；worktree 归并主项目；普通目录返回 nil。
func TestResolveProject(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	repo := ws.MakeProjectDir(filepath.Join("root", "repo"))
	wtDir := ws.MakeWorktree(repo, "wt-out", "feat")

	s := newServiceWithRules(t, ws, []ScanRule{{Group: "g", Path: root, MaxDepth: 3}}, nil)

	if p := s.ResolveProject(repo); p == nil || p.Path() != repo {
		t.Fatalf("项目根应直接命中, got %+v", p)
	}
	if p := s.ResolveProject(wtDir); p == nil || p.Path() != repo {
		t.Fatalf("worktree 应归并到主项目 %s, got %+v", repo, p)
	}
	if p := s.ResolveProject(ws.Join("plain")); p != nil {
		t.Fatalf("普通目录应返回 nil, got %+v", p)
	}
}

// TestOwnsDir 归属判定：主根/其子目录/快照内 worktree 及其子目录属于项目，
// 项目外目录与未采集 worktree 不属于。
func TestOwnsDir(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	repo := ws.MakeProjectDir(filepath.Join("root", "repo"))
	ws.Mkdir(filepath.Join("repo", "apps", "web"))
	ws.MakeWorktree(repo, "wt-out", "feat")
	ws.Mkdir(filepath.Join("wt-out", "sub"))
	// 快照内 worktree 路径经符号链接规范化（macOS /var → /private/var），测试侧同样取真实路径
	wtDir, err := filepath.EvalSymlinks(ws.Join("wt-out"))
	if err != nil {
		t.Fatal(err)
	}

	s := newServiceWithRules(t, ws, []ScanRule{{Group: "g", Path: root, MaxDepth: 3}}, nil)
	if _, _, err := s.Refresh(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		dir  string
		want bool
	}{
		{repo, true},
		{filepath.Join(repo, "apps", "web"), true},
		{wtDir, true},
		{filepath.Join(wtDir, "sub"), true},
		{ws.Join("plain"), false},
		{ws.Dir, false}, // 项目根的父目录不算领地
	}
	for _, c := range cases {
		if got := s.OwnsDir(repo, c.dir); got != c.want {
			t.Errorf("OwnsDir(%s) = %v, want %v", c.dir, got, c.want)
		}
	}
	if s.OwnsDir("/nonexistent", repo) {
		t.Error("未收录项目应返回 false")
	}
}
