package forge

import (
	"testing"

	"cube/util/gitapi"
)

// TestRepoKey clone URL 归一化对账匹配键：ssh/https/带端口/.git 尾缀/大小写各形态同键。
func TestRepoKey(t *testing.T) {
	cases := []struct {
		urls []string // 这些 URL 应归一为同一键
		want string
	}{
		{[]string{"git@github.com:heyuuu/cube.git", "https://github.com/heyuuu/cube.git", "https://github.com/Heyuuu/Cube", "git@github.com:heyuuu/cube"}, "github.com/heyuuu/cube"},
		{[]string{"https://gitea.example.com:3000/acme/app.git", "ssh://git@gitea.example.com:3000/acme/app.git", "https://gitea.example.com:3000/acme/app"}, "gitea.example.com:3000/acme/app"},
	}
	for _, c := range cases {
		for _, u := range c.urls {
			if got := RepoKey(u); got != c.want {
				t.Errorf("RepoKey(%q) = %q, want %q", u, got, c.want)
			}
		}
	}
	if got := RepoKey("not a url"); got != "" {
		t.Errorf("非法 url 应返回空串, got %q", got)
	}
}

// TestMatchNamespacePath namespace 范围判定：host 相等且 path 前缀匹配。
func TestMatchNamespacePath(t *testing.T) {
	cases := []struct {
		repoUrl, host, path string
		want                bool
	}{
		{"git@github.com:heyuuu/cube.git", "github.com", "heyuuu", true},
		{"https://github.com/Heyuuu/Cube", "GitHub.com", "/heyuuu/", true},
		{"git@github.com:heyuuu-fork/cube.git", "github.com", "heyuuu", false}, // 前缀须按段匹配
		{"git@gitlab.com:heyuuu/cube.git", "github.com", "heyuuu", false},      // host 不同
		{"", "github.com", "heyuuu", false},
	}
	for _, c := range cases {
		if got := MatchNamespacePath(c.repoUrl, c.host, c.path); got != c.want {
			t.Errorf("MatchNamespacePath(%q,%q,%q) = %v, want %v", c.repoUrl, c.host, c.path, got, c.want)
		}
	}
}

// TestReconcile 对账三分：未 clone / 孤儿 / 已 clone；孤儿含本地状态。
func TestReconcile(t *testing.T) {
	local := []LocalRepo{
		{Name: "cube", Path: "~/src/cube", RepoUrl: "git@github.com:heyuuu/cube.git", Dirty: true, Ahead: 2, Behind: 1},
		{Name: "gone", Path: "~/src/gone", RepoUrl: "https://github.com/heyuuu/gone.git"},
		{Name: "other-host", Path: "~/src/x", RepoUrl: "git@gitlab.com:a/b.git"},
	}
	remote := []gitapi.RemoteRepo{
		{Name: "cube", CloneUrl: "https://github.com/heyuuu/cube.git"},
		{Name: "fresh", CloneUrl: "https://github.com/heyuuu/fresh.git"},
	}
	got := Reconcile(local, remote)

	if len(got.Synced) != 1 || got.Synced[0].Local.Name != "cube" || !got.Synced[0].Local.Dirty || got.Synced[0].Remote.Name != "cube" {
		t.Fatalf("synced 不符: %+v", got.Synced)
	}
	if len(got.Missing) != 1 || got.Missing[0].Name != "fresh" {
		t.Fatalf("missing 不符: %+v", got.Missing)
	}
	// 纯函数不做 namespace 过滤（Service 层负责）：host 不同匹配不上的本地库同为孤儿
	if len(got.Orphan) != 2 || got.Orphan[0].Name != "gone" {
		t.Fatalf("orphan 不符: %+v", got.Orphan)
	}
}

// TestReconcileEmpty 空输入零值安全（nil 序列化 [] 由 web 层统一处理）。
func TestReconcileEmpty(t *testing.T) {
	got := Reconcile(nil, nil)
	if got.Missing != nil || got.Orphan != nil || got.Synced != nil {
		t.Fatalf("空输入应产出空结果: %+v", got)
	}
}
