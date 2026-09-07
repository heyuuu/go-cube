package forge

import (
	"testing"
	"time"

	"cube/util/gitapi"
)

func testNow() time.Time { return time.Unix(1700000000, 0) }

// TestBuildOverview 聚合：已拉取 account 出行；未拉取只出元信息；
// 孤儿 = 与拉取结果同 namespace 但不在其中（跨 namespace、未配置 namespace 的不算）。
func TestBuildOverview(t *testing.T) {
	forges := []Forge{{Host: "github.com", Kind: KindGithub}}
	accounts := []Account{
		{ForgeHost: "github.com", Username: "heyuuu", Token: "t"},
		{ForgeHost: "github.com", Username: "ghost", Token: "t"}, // 未拉取
	}
	repos := []gitapi.RemoteRepo{
		{Name: "cube", FullName: "heyuuu/cube", CloneUrl: "https://github.com/heyuuu/cube.git"},
		{Name: "fresh", FullName: "heyuuu/fresh", CloneUrl: "https://github.com/heyuuu/fresh.git"},
		{Name: "org-repo", FullName: "acme/org-repo", CloneUrl: "https://github.com/acme/org-repo.git"},
	}
	local := []LocalRepo{
		{Name: "cube", RepoUrl: "git@github.com:heyuuu/cube.git", Dirty: true},
		{Name: "gone", RepoUrl: "https://github.com/heyuuu/gone.git"}, // 同 namespace 不在结果里 → 孤儿
		{Name: "stray", RepoUrl: "git@github.com:other-ns/stray.git"}, // namespace 不在拉取结果 → 不算孤儿
		{Name: "unrelated", RepoUrl: "git@gitlab.com:x/y.git"},        // host 不命中 → 不算
		{Name: "no-remote", RepoUrl: ""},                              // 无远端 → 不参与
	}
	entries := map[string]AcctCacheEntry{
		acctCacheKey("github.com", "heyuuu"): {FetchedAt: testNow(), Repos: repos},
	}
	ov := buildOverview(forges, accounts, func(a Account) (AcctCacheEntry, bool) {
		e, ok := entries[acctCacheKey(a.ForgeHost, a.Username)]
		return e, ok
	}, local)

	if len(ov.Accounts) != 2 || ov.Accounts[1].RepoCount != 0 || ov.Accounts[0].FetchedAt.IsZero() {
		t.Fatalf("account 元信息不符: %+v", ov.Accounts)
	}
	byStatus := map[string][]RepoRow{}
	for _, r := range ov.Rows {
		byStatus[r.Status] = append(byStatus[r.Status], r)
	}
	if len(byStatus[StatusMissing]) != 2 {
		t.Fatalf("missing 应 2 条（fresh + org-repo）: %+v", byStatus[StatusMissing])
	}
	if byStatus[StatusMissing][0].Owner != "acme" && byStatus[StatusMissing][1].Owner != "acme" {
		t.Fatalf("missing 行应带 owner: %+v", byStatus[StatusMissing])
	}
	if len(byStatus[StatusSynced]) != 1 || byStatus[StatusSynced][0].Local == nil || byStatus[StatusSynced][0].Local.Name != "cube" || byStatus[StatusSynced][0].Owner != "heyuuu" {
		t.Fatalf("synced 行不符: %+v", byStatus[StatusSynced])
	}
	// 孤儿只认「同 namespace 且不在拉取结果」：gone 算，stray/unrelated/no-remote 都不算
	if len(byStatus[StatusOrphan]) != 1 || byStatus[StatusOrphan][0].Local.Name != "gone" || byStatus[StatusOrphan][0].Owner != "heyuuu" {
		t.Fatalf("orphan 行不符: %+v", byStatus[StatusOrphan])
	}
}
