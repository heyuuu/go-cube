package git

import (
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
	// HEAD -> main → local；tag: v1 → tag
	if len(c1.Refs) != 2 || c1.Refs[0] != (CommitRef{"main", "local"}) || c1.Refs[1] != (CommitRef{"v1", "tag"}) {
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
	if len(all[0].Refs) == 0 || all[0].Refs[0].Name != "main" || all[0].Refs[0].Kind != "local" {
		t.Errorf("首条应带 main(local) 装饰: %v", all[0].Refs)
	}
}
