package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"cube/internal/testfixture"
)

func TestParseLogFields(t *testing.T) {
	in := "aaa111\x1ff1a111\x1fbbb111 ccc111\x1f张三\x1f1700000000\x1fHEAD -> main, tag: v1\x1f提交一\n" +
		"junk\n" + // 非法行（分隔字段不足）跳过
		"bbb222\x1ff2b222\x1f\x1f李四\x1f1700000100\x1f\x1ffeat: 新功能"
	commits := parseLogFields(in)
	if len(commits) != 2 {
		t.Fatalf("应解析 2 条, got %d", len(commits))
	}
	c1 := commits[0]
	if c1.Sha != "aaa111" || c1.ShortSha != "f1a111" || c1.Subject != "提交一" || c1.Author != "张三" || c1.Timestamp != 1700000000 {
		t.Errorf("c1 字段不符: %+v", c1)
	}
	if len(c1.Parents) != 2 || c1.Parents[0] != "bbb111" || c1.Parents[1] != "ccc111" {
		t.Errorf("c1 parents 不符: %v", c1.Parents)
	}
	// HEAD -> main, tag: v1 → [main, v1]
	if len(c1.Refs) != 2 || c1.Refs[0] != "main" || c1.Refs[1] != "v1" {
		t.Errorf("c1 refs 不符: %v", c1.Refs)
	}
	if len(commits[1].Parents) != 0 {
		t.Errorf("根提交 parents 应为空: %v", commits[1].Parents)
	}
}

func TestCommitsPage(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "main", EmptyCommitCount: 5})

	all, err := CommitsPage(repo, false, "", 0, 3)
	if err != nil {
		t.Fatalf("CommitsPage 报错: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("limit=3 应返回 3 条, got %d", len(all))
	}
	page2, _ := CommitsPage(repo, false, "", 3, 3)
	if len(page2) != 2 {
		t.Fatalf("第二页应剩 2 条, got %d", len(page2))
	}
	if page2[0].Sha == all[0].Sha {
		t.Error("分页应前进")
	}
	// 第一条是最新的：parents 指向后一条
	if all[0].Parents[0] != all[1].Sha {
		t.Errorf("链式 parents 不符: %v -> %v", all[0].Parents, all[1].Sha)
	}
	if len(all[0].Refs) == 0 || all[0].Refs[0] != "main" {
		t.Errorf("首条应带 main 装饰: %v", all[0].Refs)
	}
}

func TestLoadRepoStatus(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "main", EmptyCommitCount: 2})

	st, err := LoadRepoStatus(repo)
	if err != nil {
		t.Fatalf("LoadRepoStatus 报错: %v", err)
	}
	if st.Branch != "main" || st.Sha == "" || st.Dirty {
		t.Errorf("干净仓库字段不符: %+v", st)
	}

	// untracked + staged + unstaged 三类各一个
	os.WriteFile(filepath.Join(repo, "new.txt"), []byte("x"), 0o644)     // untracked
	os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("a"), 0o644) // 将变 staged
	_ = exec.Command("git", "-C", repo, "add", "tracked.txt").Run()
	os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("ab"), 0o644) // 又变 unstaged

	st2, _ := LoadRepoStatus(repo)
	if st2.Staged != 1 || st2.Unstaged != 1 || st2.Untracked != 1 || !st2.Dirty {
		t.Errorf("三类变更计数不符: %+v", st2)
	}

	// 非 git 目录：零值 + nil
	st3, err := LoadRepoStatus(ws.Join("plain"))
	if err != nil || st3.Dirty {
		t.Errorf("非 git 目录应返回零值+nil: %+v, err=%v", st3, err)
	}
}
