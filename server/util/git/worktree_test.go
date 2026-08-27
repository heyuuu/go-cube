package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cube/internal/testfixture"
)

// runGit 测试内直接执行 git（建仓/造状态；规则 14 允许测试绕过封装）。
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s 失败: %v\n%s", strings.Join(args, " "), err, out)
	}
}

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

// WorktreePrune 清理目录已删的 worktree 元数据记录（幂等、不碰现存 worktree）。
func TestWorktreePrune(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo")
	ws.MakeWorktree(repo, "wt-hot", "hotfix")
	ws.MakeWorktree(repo, "wt-gone", "gone")

	if err := os.RemoveAll(ws.Join("wt-gone")); err != nil {
		t.Fatalf("删除 worktree 目录失败: %v", err)
	}

	if err := WorktreePrune(repo); err != nil {
		t.Fatalf("WorktreePrune 报错: %v", err)
	}

	list, err := WorktreeList(repo)
	if err != nil {
		t.Fatalf("WorktreeList 报错: %v", err)
	}
	if len(list) != 2 { // 主目录 + wt-hot；wt-gone 元数据应已被清
		t.Fatalf("prune 后应剩 2 个工作副本, got %d (%+v)", len(list), list)
	}
}

// WorktreeAdd 三种形态：新建分支 / 检出已有分支 / detached。
func TestWorktreeAdd(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Tags: []string{"v1.0"}})
	runGit(t, repo, "branch", "existing")

	cases := []struct {
		name       string
		branch     string
		commitish  string
		wantBranch string
	}{
		{"新建分支", "feat-new", "", "feat-new"},
		{"新建分支-指定基点tag", "feat-tag", "v1.0", "feat-tag"},
		{"检出已有分支", "existing", "", "existing"},
		{"detached", "", "existing", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := ws.Join("wt-" + tc.name)
			wt, err := WorktreeAdd(repo, target, tc.branch, tc.commitish)
			if err != nil {
				t.Fatalf("WorktreeAdd 报错: %v", err)
			}
			canonical, _ := filepath.EvalSymlinks(target)
			if wt.Path != canonical {
				t.Errorf("返回路径不符: want %s, got %s", canonical, wt.Path)
			}
			if wt.Branch != tc.wantBranch {
				t.Errorf("检出分支不符: want %q, got %q", tc.wantBranch, wt.Branch)
			}
			if tc.branch == "" && !wt.Detached {
				t.Errorf("branch 为空应 detached: %+v", wt)
			}
		})
	}
}

// 已被检出的分支（主目录或其它 worktree）不允许再建 worktree 检出。
func TestWorktreeAddCheckedOutRefused(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo") // 主目录检出默认分支
	def := CurrentBranch(repo)

	if _, err := WorktreeAdd(repo, ws.Join("wt-main"), def, ""); err == nil {
		t.Fatalf("检出主目录当前分支应被拒绝")
	} else if !strings.Contains(err.Error(), "已被工作副本检出") {
		t.Errorf("错误应为中文检出冲突提示, got: %v", err)
	}

	ws.MakeWorktree(repo, "wt-hot", "hotfix")
	if _, err := WorktreeAdd(repo, ws.Join("wt-hot2"), "hotfix", ""); err == nil {
		t.Fatalf("检出其它 worktree 已占用的分支应被拒绝")
	}
}

// WorktreeRemove：干净副本直接删；脏副本非 force 拒绝、force 删净。
func TestWorktreeRemove(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo")

	clean := ws.MakeWorktree(repo, "wt-clean", "clean")
	if err := WorktreeRemove(repo, clean, false); err != nil {
		t.Fatalf("干净副本删除应成功: %v", err)
	}

	dirty := ws.MakeWorktree(repo, "wt-dirty", "dirty")
	writeUntracked(t, filepath.Join(dirty, "new.txt"))
	if err := WorktreeRemove(repo, dirty, false); err == nil {
		t.Fatalf("脏副本非 force 删除应被拒绝")
	}
	if err := WorktreeRemove(repo, dirty, true); err != nil {
		t.Fatalf("脏副本 force 删除应成功: %v", err)
	}

	if err := WorktreePrune(repo); err != nil {
		t.Fatalf("prune 报错: %v", err)
	}
	list, err := WorktreeList(repo)
	if err != nil {
		t.Fatalf("WorktreeList 报错: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("删除+prune 后应只剩主目录, got %d (%+v)", len(list), list)
	}
}

func writeUntracked(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("uncommitted"), 0o644); err != nil {
		t.Fatalf("写未跟踪文件失败: %v", err)
	}
}
