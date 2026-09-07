// remote 查询的测试（被测实现在 remote.go；directGit/directAddRemote 定义在 refs_test.go，同包共用）。
package git

import (
	"reflect"
	"testing"
)

// TestRemoteUrl_NonRepo 非仓库目录返回空值不报错（降级约定）。
func TestRemoteUrl_NonRepo(t *testing.T) {
	ws := newTestWorkspace(t)
	dir := ws.Mkdir("not-a-repo")
	url, err := RemoteUrl(dir)
	if err != nil || url != "" {
		t.Fatalf("非仓库 RemoteUrl 应返回 (\"\",nil)，实际 (%q,%v)", url, err)
	}
}

// TestRemoteUrl_WithRemote 有 origin remote 的仓库能读出 URL。
func TestRemoteUrl_WithRemote(t *testing.T) {
	ws := newTestWorkspace(t)
	// remote 指向另一个本地路径（合法的本地 remote）
	dir := ws.MakeGitRepoWith("repo", GitRepoSpec{
		RemoteUrl: "/tmp/some-remote.git",
	})
	url, err := RemoteUrl(dir)
	if err != nil {
		t.Fatalf("RemoteUrl 出错: %v", err)
	}
	if url != "/tmp/some-remote.git" {
		t.Fatalf("RemoteUrl = %q，期望 /tmp/some-remote.git", url)
	}
}

// TestRemoteUrl_NoOrigin 无 origin remote 时返回空值不报错。
func TestRemoteUrl_NoOrigin(t *testing.T) {
	ws := newTestWorkspace(t)
	dir := ws.MakeGitRepo("repo")
	url, err := RemoteUrl(dir)
	if err != nil || url != "" {
		t.Fatalf("无 origin 时应返回空，实际 (%q,%v)", url, err)
	}
}

// TestRemotes_WithMultipleRemotes 多 remote 仓库返回全部，按名字排序。
func TestRemotes_WithMultipleRemotes(t *testing.T) {
	ws := newTestWorkspace(t)
	// 先建仓库，再加额外 remote
	dir := ws.MakeGitRepoWith("repo", GitRepoSpec{
		RemoteUrl: "https://github.com/a/b.git",
	})
	// fixture 只支持 origin，upstream 用 git 命令直接加
	directAddRemote(t, dir, "upstream", "https://github.com/upstream/b.git")

	remotes, err := Remotes(dir)
	if err != nil {
		t.Fatalf("Remotes 出错: %v", err)
	}
	if len(remotes) != 2 {
		t.Fatalf("Remotes 数量 = %d，期望 2：%v", len(remotes), remotes)
	}
	// 按 name 查找
	byName := map[string]Remote{}
	for _, r := range remotes {
		byName[r.Name] = r
	}
	if r, ok := byName["origin"]; !ok || r.Fetch != "https://github.com/a/b.git" {
		t.Fatalf("origin remote 异常：%v", byName["origin"])
	}
	if r, ok := byName["upstream"]; !ok || r.Fetch != "https://github.com/upstream/b.git" {
		t.Fatalf("upstream remote 异常：%v", byName["upstream"])
	}
}

// TestRemotes_SeparatePushUrl 配置独立 pushurl 时 Fetch 与 Push 不同。
func TestRemotes_SeparatePushUrl(t *testing.T) {
	ws := newTestWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", GitRepoSpec{
		RemoteUrl: "https://github.com/a/b.git",
	})
	directGit(t, dir, "remote", "set-url", "--push", "origin", "git@github.com:a/b.git")

	remotes, err := Remotes(dir)
	if err != nil || len(remotes) != 1 {
		t.Fatalf("Remotes = (%v, %v)，期望单个 remote", remotes, err)
	}
	r := remotes[0]
	if r.Fetch != "https://github.com/a/b.git" || r.Push != "git@github.com:a/b.git" {
		t.Fatalf("pushurl remote 异常：%+v", r)
	}
}

// TestParseRemotesVerbose 解析 remote -v 输出：同名 fetch/push 合并、垃圾行跳过、按名字排序。
func TestParseRemotesVerbose(t *testing.T) {
	out := "origin\tgit@github.com:a/b.git (fetch)\n" +
		"origin\tgit@github.com:a/b.git (push)\n" +
		"upstream\thttps://x/y.git (fetch)\n" +
		"upstream\tgit@x:y.git (push)\n" +
		"garbage-no-tab\n" +
		"\n"

	got := parseRemotesVerbose(out)
	want := []Remote{
		{Name: "origin", Fetch: "git@github.com:a/b.git", Push: "git@github.com:a/b.git"},
		{Name: "upstream", Fetch: "https://x/y.git", Push: "git@x:y.git"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseRemotesVerbose = %v，期望 %v", got, want)
	}
	if parseRemotesVerbose("") != nil {
		t.Fatalf("空输入应返回 nil")
	}
}
