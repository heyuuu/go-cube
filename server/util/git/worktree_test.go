package git

import (
	"path/filepath"
	"testing"

	"cube/internal/testfixture"
)

func TestParseWorktreePorcelain(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Worktree
	}{
		{
			name: "单工作副本（主目录）",
			in:   "worktree /Users/x/proj\nHEAD 1a2b3c4\nbranch refs/heads/main\n",
			want: []Worktree{{Path: "/Users/x/proj", Head: "1a2b3c4", Branch: "main"}},
		},
		{
			name: "主目录 + linked worktree",
			in: "worktree /Users/x/proj\nHEAD 1a2b3c4\nbranch refs/heads/main\n\n" +
				"worktree /Users/x/wt-hot\nHEAD 9f8e7d6\nbranch refs/heads/hotfix\n",
			want: []Worktree{
				{Path: "/Users/x/proj", Head: "1a2b3c4", Branch: "main"},
				{Path: "/Users/x/wt-hot", Head: "9f8e7d6", Branch: "hotfix"},
			},
		},
		{
			name: "detached + bare",
			in:   "worktree /Users/x/proj\nHEAD 1a2b3c4\ndetached\n\nworktree /Users/x/proj.git\nbare\n",
			want: []Worktree{
				{Path: "/Users/x/proj", Head: "1a2b3c4", Detached: true},
				{Path: "/Users/x/proj.git", Bare: true},
			},
		},
		{name: "空输出", in: "", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWorktreePorcelain(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("数量不符: want %d, got %d (%+v)", len(tt.want), len(got), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("第 %d 个不符: want %+v, got %+v", i, tt.want[i], got[i])
				}
			}
		})
	}
}

func TestWorktreeList(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo")

	// 加一个 linked worktree（独立分支避免与主目录检出冲突）
	wtDir := ws.MakeWorktree(repo, "wt-hot", "hotfix")

	list, err := WorktreeList(repo)
	if err != nil {
		t.Fatalf("WorktreeList 报错: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("应有 2 个工作副本, got %d (%+v)", len(list), list)
	}
	mainWt, hotWt := list[0], list[1]
	// git 输出的路径经符号链接规范化（macOS 上 /var → /private/var），比较前同样求值
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("解析真实路径失败: %v", err)
	}
	canonicalWtDir, err := filepath.EvalSymlinks(wtDir)
	if err != nil {
		t.Fatalf("解析真实路径失败: %v", err)
	}
	if mainWt.Path != canonicalRepo || mainWt.Branch == "" || mainWt.Head == "" {
		t.Errorf("主目录字段不完整: %+v", mainWt)
	}
	if hotWt.Path != canonicalWtDir || hotWt.Branch != "hotfix" {
		t.Errorf("linked worktree 字段不符: %+v", hotWt)
	}

	// 非 git 目录报错
	if _, err := WorktreeList(ws.Join("plain")); err == nil {
		t.Error("非 git 目录应报错")
	}
}

// 从 worktree 目录内调用应得到与主目录一致的列表（1032 归并链路依赖：
// 命中 worktree 目录时顺藤定位主仓库、枚举全部目标）。
func TestWorktreeListFromWorktreeDir(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo")
	wtDir := ws.MakeWorktree(repo, "wt-feat", "feat")

	fromMain, err := WorktreeList(repo)
	if err != nil {
		t.Fatalf("主目录调用报错: %v", err)
	}
	fromWorktree, err := WorktreeList(wtDir)
	if err != nil {
		t.Fatalf("worktree 目录调用报错: %v", err)
	}
	if len(fromMain) != 2 || len(fromWorktree) != 2 {
		t.Fatalf("两侧都应有 2 个工作副本: main=%d worktree=%d", len(fromMain), len(fromWorktree))
	}
	for i := range fromMain {
		if fromMain[i] != fromWorktree[i] {
			t.Errorf("第 %d 个副本两侧不一致: main=%+v worktree=%+v", i, fromMain[i], fromWorktree[i])
		}
	}
}
