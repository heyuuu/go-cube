package cmd

import (
	"reflect"
	"testing"

	"cube/util/gogit"
)

// TestBuildRemoteBranchMap 验证远程分支按短名组织成 map[branch]set[remote]，
// 含斜杠分支名（feature/fix-bug）在多 remote 下的归属。
func TestBuildRemoteBranchMap(t *testing.T) {
	m := buildRemoteBranchMap([]gogit.RemoteBranch{
		{Remote: "origin", Branch: "master"},
		{Remote: "origin", Branch: "feature/fix-bug"},
		{Remote: "upstream", Branch: "feature/fix-bug"},
	})

	want := map[string]map[string]bool{
		"master":          {"origin": true},
		"feature/fix-bug": {"origin": true, "upstream": true},
	}
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("buildRemoteBranchMap = %v，期望 %v", m, want)
	}
}

// TestPickSharedBranches 验证交集选取：斜杠本地分支正确命中同名远程分支，
// 所有 remote 都没有的本地分支被排除，结果按名排序。
func TestPickSharedBranches(t *testing.T) {
	local := []string{"feature/fix-bug", "master", "local-only"}
	m := map[string]map[string]bool{
		"master":          {"origin": true},
		"feature/fix-bug": {"origin": true, "upstream": true},
	}

	got := pickSharedBranches(local, m)
	want := []string{"feature/fix-bug", "master"} // local-only 无远程同名分支，排除
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pickSharedBranches = %v，期望 %v", got, want)
	}
}
