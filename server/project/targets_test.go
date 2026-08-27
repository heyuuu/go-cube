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
		label, relPath string
		kind           TargetKind
	}
	want := []wantItem{
		{"server", "server", KindWorkspace},
		{"web", "web", KindWorkspace},
		{"hotfix", "/x/wt-hot", KindWorktree}, // worktree 根不可 stat，被存在性过滤
		{"wt-detached", "/x/wt-detached", KindWorktree},
		{"wt-a", "/x/wt-a", KindWorktree},
		{"wt-b", "/x/wt-b", KindWorktree},
	}
	if len(got) != len(want) {
		t.Fatalf("应返回 %d 个目标, got %d (%+v)", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i].Label != w.label || got[i].Kind != w.kind {
			t.Errorf("第 %d 个不符: want %s/%s, got %s/%s", i, w.label, w.kind, got[i].Label, got[i].Kind)
		}
		if w.relPath != "" && w.relPath[0] == '/' && got[i].Path != w.relPath {
			t.Errorf("第 %d 个路径不符: want %s, got %s", i, w.relPath, got[i].Path)
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

// TestTargetEntriesWorktreeWorkspaces worktree 根下的 workspace 同样展开（Kind 恒为 workspace），
// 排序紧跟其 worktree 根。
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
	if got[0].Kind != KindWorktree || got[0].Label != "feat" {
		t.Errorf("worktree 根目标不符: %+v", got[0])
	}
	if got[1].Kind != KindWorkspace || got[1].Path != filepath.Join(wtRoot, "app") || got[1].Label != "app" {
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
	if len(targets) != 1 || targets[0].Path != repo || targets[0].Label != "根目录" || targets[0].Kind != KindRoot {
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
	if targets[0].Path != repo || targets[0].Kind != KindRoot {
		t.Fatalf("根目录目标不符: %+v", targets[0])
	}
	if targets[1].Kind != KindWorkspace || targets[1].Path != filepath.Join(repo, "server") || targets[1].Label != "服务端" {
		t.Fatalf("workspace 目标不符: %+v", targets[1])
	}
	canonicalWt, err := filepath.EvalSymlinks(ws.Join("wt-hot"))
	if err != nil {
		t.Fatalf("解析真实路径失败: %v", err)
	}
	if targets[2].Path != canonicalWt || targets[2].Label != "hotfix" || targets[2].Branch != "hotfix" || targets[2].Kind != KindWorktree {
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
