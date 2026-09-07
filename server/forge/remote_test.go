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

// TestRepoNamespacePath namespace 段推导（owner 归组与孤儿判定的依据）。
func TestRepoNamespacePath(t *testing.T) {
	cases := []struct {
		repoUrl string
		want    string
	}{
		{"git@github.com:heyuuu/cube.git", "heyuuu"},
		{"https://gitee.com/CE_LBT/edu-web.git", "ce_lbt"},
		{"https://gitea.example.com:3000/acme/app", "acme"},
		{"", ""},
		{"not a url", ""},
	}
	for _, c := range cases {
		if got := RepoNamespacePath(c.repoUrl); got != c.want {
			t.Errorf("RepoNamespacePath(%q) = %q, want %q", c.repoUrl, got, c.want)
		}
	}
}

// TestReconcile 对账二分：未 clone / 已 clone；孤儿判定在 buildOverview（namespace 口径），不走此函数。
func TestReconcile(t *testing.T) {
	local := []LocalRepo{
		{Name: "cube", Path: "~/src/cube", RepoUrl: "git@github.com:heyuuu/cube.git", Dirty: true, Ahead: 2, Behind: 1},
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
}

// TestReconcileEmpty 空输入零值安全（nil 序列化 [] 由 web 层统一处理）。
func TestReconcileEmpty(t *testing.T) {
	got := Reconcile(nil, nil)
	if got.Missing != nil || got.Synced != nil {
		t.Fatalf("空输入应产出空结果: %+v", got)
	}
}
