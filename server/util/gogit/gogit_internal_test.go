package gogit

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
)

func TestRefShortName(t *testing.T) {
	cases := []struct {
		name string
		ref  plumbing.ReferenceName
		want string
	}{
		{"本地分支", "refs/heads/master", "master"},
		{"本地分支带斜杠", "refs/heads/feature/x", "feature/x"},
		{"远程分支", "refs/remotes/origin/main", "origin/main"},
		{"远程分支带斜杠", "refs/remotes/origin/feature/y", "origin/feature/y"},
		{"远程 HEAD", "refs/remotes/origin/HEAD", "origin/HEAD"},
		{"tag", "refs/tags/v1.0", "v1.0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := refShortName(c.ref)
			if got != c.want {
				t.Fatalf("refShortName(%s) = %q，期望 %q", c.ref, got, c.want)
			}
		})
	}
}

func TestSplitRemoteRef(t *testing.T) {
	cases := []struct {
		name       string
		ref        plumbing.ReferenceName
		wantRemote string
		wantBranch string
		wantOK     bool
	}{
		{"普通远程分支", "refs/remotes/origin/master", "origin", "master", true},
		{"分支名含斜杠", "refs/remotes/origin/feature/x", "origin", "feature/x", true},
		{"HEAD 应 ok=false", "refs/remotes/origin/HEAD", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			remote, branch, ok := splitRemoteRef(c.ref)
			if ok != c.wantOK || remote != c.wantRemote || branch != c.wantBranch {
				t.Fatalf("splitRemoteRef(%s) = (%q,%q,%v)，期望 (%q,%q,%v)",
					c.ref, remote, branch, ok, c.wantRemote, c.wantBranch, c.wantOK)
			}
		})
	}
}

func TestSplitRemoteBranchShortName(t *testing.T) {
	cases := []struct {
		in         string
		wantRemote string
		wantBranch string
		wantOK     bool
	}{
		{"origin/master", "origin", "master", true},
		{"origin/feature/x", "origin", "feature/x", true},
		{"upstream/main", "upstream", "main", true},
		{"master", "", "", false}, // 无 remote 前缀
		{"", "", "", false},       // 空
		{"/foo", "", "", false},   // remote 名为空（idx<=0）
	}
	for _, c := range cases {
		remote, branch, ok := splitRemoteBranchShortName(c.in)
		if ok != c.wantOK || remote != c.wantRemote || branch != c.wantBranch {
			t.Errorf("splitRemoteBranchShortName(%q) = (%q,%q,%v)，期望 (%q,%q,%v)",
				c.in, remote, branch, ok, c.wantRemote, c.wantBranch, c.wantOK)
		}
	}
}

func TestStripRemotePrefix(t *testing.T) {
	cases := map[string]string{
		"origin/master": "master",
		"upstream/main": "main",
		"origin/feat/x": "feat/x",
		"master":        "master",      // 无前缀原样返回
		"fork/branch":   "fork/branch", // 未识别前缀原样返回
	}
	for in, want := range cases {
		got := stripRemotePrefix(in)
		if got != want {
			t.Errorf("stripRemotePrefix(%q) = %q，期望 %q", in, got, want)
		}
	}
}
