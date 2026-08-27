package project

import (
	"path/filepath"
	"testing"

	"cube/internal/testfixture"
	"cube/project/gitcache"
)

// TestWorktreeTargets 展示名规则：分支名优先，detached（空分支）/ 同分支冲突回退目录名。
func TestWorktreeTargets(t *testing.T) {
	info := &gitcache.Entry{Worktrees: []gitcache.WorktreeInfo{
		{Path: "/x/wt-hot", Branch: "hotfix"},
		{Path: "/x/wt-detached", Branch: "", Detached: true},
		{Path: "/x/wt-a", Branch: "feat"},
		{Path: "/x/wt-b", Branch: "feat"}, // 与 wt-a 同分支，双双回退目录名
	}}

	got := worktreeTargets(info)
	if len(got) != 4 {
		t.Fatalf("应返回 4 个目标, got %d (%+v)", len(got), got)
	}
	want := []struct{ path, label string }{
		{"/x/wt-hot", "hotfix"},
		{"/x/wt-detached", "wt-detached"},
		{"/x/wt-a", "wt-a"},
		{"/x/wt-b", "wt-b"},
	}
	for i, w := range want {
		if got[i].Path != w.path || got[i].Label != w.label {
			t.Errorf("第 %d 个不符: want %s/%s, got %s/%s", i, w.path, w.label, got[i].Path, got[i].Label)
		}
	}

	if worktreeTargets(nil) != nil {
		t.Error("nil entry 应返回 nil")
	}
}

// TestOpenTargets 根目录在前 + 快照内 worktree 跟随；未采集/无 worktree 项目只有根目录。
func TestOpenTargets(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	repo := ws.MakeProjectDir(filepath.Join("root", "repo"))

	s := newServiceWithRules(t, ws, []ScanRule{{Group: "g", Path: root, MaxDepth: 3}}, nil)

	// 未采集快照时只有根目录目标
	targets := s.OpenTargets(repo)
	if len(targets) != 1 || targets[0].Path != repo || targets[0].Label != "根目录" {
		t.Fatalf("无快照应只有根目录目标: %+v", targets)
	}

	// 建两个 worktree（detached 场景的回退逻辑已由 TestWorktreeTargets 覆盖，这里验证快照链路），
	// 真实采集后目标列表 = 根目录 + worktrees
	ws.MakeWorktree(repo, "wt-hot", "hotfix")
	if _, _, err := s.Refresh(); err != nil {
		t.Fatalf("Refresh 失败: %v", err)
	}

	targets = s.OpenTargets(repo)
	if len(targets) != 2 {
		t.Fatalf("应有 2 个目标（根 + 1 worktree), got %+v", targets)
	}
	if targets[0].Path != repo || targets[0].Label != "根目录" {
		t.Fatalf("根目录目标不符: %+v", targets[0])
	}
	canonicalWt, err := filepath.EvalSymlinks(ws.Join("wt-hot"))
	if err != nil {
		t.Fatalf("解析真实路径失败: %v", err)
	}
	if targets[1].Path != canonicalWt || targets[1].Label != "hotfix" || targets[1].Branch != "hotfix" {
		t.Fatalf("worktree 目标不符: %+v", targets[1])
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
