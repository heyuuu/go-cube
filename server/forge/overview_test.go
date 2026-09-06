package forge

import (
	"testing"
	"time"

	"cube/util/gitapi"
)

func testNow() time.Time { return time.Unix(1700000000, 0) }

// TestBuildOverview 聚合：已拉取 namespace 出行；未拉取只出元信息；孤儿跨 namespace 去重。
func TestBuildOverview(t *testing.T) {
	namespaces := []Namespace{
		{ForgeHost: "github.com", Path: "heyuuu", Type: "personal"},
		{ForgeHost: "github.com", Path: "acme", Type: "org"}, // 未拉取
	}
	repos := []gitapi.RemoteRepo{
		{Name: "cube", CloneUrl: "https://github.com/heyuuu/cube.git"},
		{Name: "fresh", CloneUrl: "https://github.com/heyuuu/fresh.git"},
	}
	local := []LocalRepo{
		{Name: "cube", RepoUrl: "git@github.com:heyuuu/cube.git", Dirty: true},
		{Name: "gone", RepoUrl: "https://github.com/heyuuu/gone.git"},
		{Name: "unrelated", RepoUrl: "git@gitlab.com:x/y.git"},
	}
	entries := map[string]NsCacheEntry{
		nsCacheKey("github.com", "heyuuu"): {FetchedAt: testNow(), Repos: repos},
	}
	ov := buildOverview(namespaces, func(ns Namespace) (NsCacheEntry, bool) {
		e, ok := entries[nsCacheKey(ns.ForgeHost, ns.Path)]
		return e, ok
	}, local)

	if len(ov.Namespaces) != 2 || ov.Namespaces[1].RepoCount != 0 || ov.Namespaces[0].FetchedAt.IsZero() {
		t.Fatalf("ns 元信息不符: %+v", ov.Namespaces)
	}
	byStatus := map[string][]RepoRow{}
	for _, r := range ov.Rows {
		byStatus[r.Status] = append(byStatus[r.Status], r)
	}
	if len(byStatus[StatusMissing]) != 1 || byStatus[StatusMissing][0].Repo.Name != "fresh" {
		t.Fatalf("missing 行不符: %+v", byStatus[StatusMissing])
	}
	if len(byStatus[StatusSynced]) != 1 || byStatus[StatusSynced][0].Local == nil || byStatus[StatusSynced][0].Local.Name != "cube" {
		t.Fatalf("synced 行不符: %+v", byStatus[StatusSynced])
	}
	if len(byStatus[StatusOrphan]) != 1 || byStatus[StatusOrphan][0].Local.Name != "gone" {
		t.Fatalf("orphan 行不符: %+v", byStatus[StatusOrphan])
	}
}
