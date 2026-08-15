package cmd

import (
	"testing"
)

// TestPullBranchLabel 分支多选文案：当前分支 * 标记 + 同步状态描述。
func TestPullBranchLabel(t *testing.T) {
	tests := []struct {
		name          string
		d             branchDiff
		currentBranch string
		want          string
	}{
		{"当前分支 + 落后", branchDiff{Branch: "develop", Behind: 2}, "develop", "branch: * develop  (落后 2)"},
		{"普通分支 + 领先", branchDiff{Branch: "master", Ahead: 1}, "develop", "branch: master  (本地领先 1)"},
		{"分叉分支", branchDiff{Branch: "feature", Ahead: 2, Behind: 3}, "develop", "branch: feature  (分叉：本地领先 2 / 落后 3)"},
		{"已同步", branchDiff{Branch: "release"}, "develop", "branch: release  (已同步)"},
		{"detached HEAD 无当前分支标记", branchDiff{Branch: "develop", Behind: 1}, "", "branch: develop  (落后 1)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pullBranchLabel(tt.d, tt.currentBranch); got != tt.want {
				t.Errorf("pullBranchLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPullDefaultBranches 默认勾选逻辑：优先远端领先的分支；
// 都不落后时退回当前分支；detached HEAD（无当前分支）时为空。
func TestPullDefaultBranches(t *testing.T) {
	candidates := []branchDiff{
		{Branch: "develop", Ahead: 2, Behind: 3},
		{Branch: "master"},
		{Branch: "feature", Behind: 1},
	}

	// 有落后分支：只勾落后的 develop/feature
	got := pullDefaultBranches(candidates, "master")
	if len(got) != 2 || got[0].Branch != "develop" || got[1].Branch != "feature" {
		t.Errorf("应默认勾选落后分支 develop/feature，实际 %v", got)
	}

	// 都不落后：退回当前分支
	got = pullDefaultBranches([]branchDiff{{Branch: "master"}, {Branch: "develop"}}, "develop")
	if len(got) != 1 || got[0].Branch != "develop" {
		t.Errorf("无落后时应默认勾选当前分支 develop，实际 %v", got)
	}

	// 都不落后且 detached HEAD：无默认
	got = pullDefaultBranches([]branchDiff{{Branch: "master"}}, "")
	if len(got) != 0 {
		t.Errorf("detached HEAD 时不应有默认勾选，实际 %v", got)
	}
}
