package cmd

import (
	"reflect"
	"testing"

	"cube/util/git"
)

// TestBuildRemoteBranchMap 验证远程分支按短名组织成 map[branch]set[remote]，
// 含斜杠分支名（feature/fix-bug）在多 remote 下的归属。
func TestBuildRemoteBranchMap(t *testing.T) {
	m := buildRemoteBranchMap([]git.RemoteBranch{
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

// TestBuildStatusRowsFromDiffs 验证由预计算差距构造宽表行：
// 同步 ✓、落后 +N、领先 -N、remote 无该分支 -、当前分支 * 标记。
func TestBuildStatusRowsFromDiffs(t *testing.T) {
	remotes := []git.Remote{{Name: "origin"}, {Name: "gitee"}}
	branches := []string{"develop", "master"}
	diffs := []branchRemoteDiff{
		{Branch: "develop", Remote: "origin", Behind: 2},
		{Branch: "develop", Remote: "gitee", Ahead: 0, Behind: 0},
		{Branch: "master", Remote: "origin", Ahead: 1},
		// master 在 gitee 上无同名分支 → 该格 "-"
	}

	rows := buildStatusRowsFromDiffs(diffs, branches, remotes, "develop")

	want := [][]string{
		{"* develop", "-2", "✓"},
		{"master", "+1", "-"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("buildStatusRowsFromDiffs = %v，期望 %v", rows, want)
	}
}
