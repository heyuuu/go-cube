package web

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cube/util/git"
	"cube/workbench"
)

func TestWorkbenchInfo(t *testing.T) {
	env := newTestEnv(t)
	// newTestEnv 的两个项目目录就是真实 git 仓库
	repo := env.ws.Join("g1/proj1")

	var got struct {
		Root      string `json:"root"`
		Worktrees []struct {
			Path   string `json:"path"`
			Branch string `json:"branch"`
			Head   string `json:"head"`
		} `json:"worktrees"`
		DefaultBranch string `json:"defaultBranch"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/info?path="+repo)), &got)
	if got.Root != repo {
		t.Errorf("root 应为 %s, got %q", repo, got.Root)
	}
	if len(got.Worktrees) != 1 || got.Worktrees[0].Path != repo {
		t.Fatalf("应有且仅有主工作副本: %+v", got.Worktrees)
	}
	if got.Worktrees[0].Branch == "" || got.Worktrees[0].Head == "" {
		t.Errorf("主工作副本 branch/head 不应为空: %+v", got.Worktrees[0])
	}

	// 非 git 目录（不能落在任何 git 仓库内，否则向上探测会命中外层仓库根）→ ok=false
	if r := getJSON(t, env.url("/api/workbench/info?path=/definitely/not/a/repo")); r.Ok {
		t.Error("非 git 目录应报错")
	}
}

func TestWorkbenchRefs(t *testing.T) {
	env := newTestEnv(t)
	repo := env.ws.Join("g2/proj2")

	var got struct {
		Locals  []string `json:"locals"`
		Current string   `json:"current"`
		Remotes []any    `json:"remotes"`
		Tags    []string `json:"tags"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/refs?path="+repo)), &got)
	if len(got.Locals) == 0 {
		t.Error("本地分支不应为空")
	}
	if got.Current == "" {
		t.Error("当前分支不应为空")
	}
	// 无 remote/tag 的 fixture：remotes/tags 是 nil 切片，envelope 应序列化为 []
	if got.Remotes == nil || got.Tags == nil {
		t.Errorf("nil 切片应序列化为 [], got remotes=%v tags=%v", got.Remotes, got.Tags)
	}
}

// TestWorkbenchSourceParse 校验 TreeSource 解析纯函数（HTTP 契约的一部分）。
func TestWorkbenchSourceParse(t *testing.T) {
	if _, err := workbench.ParseTreeSource("branch", "main"); err == nil {
		t.Error("未知 type 应报错")
	}
	if _, err := workbench.ParseTreeSource("commit", ""); err == nil {
		t.Error("空 id 应报错")
	}
	src, err := workbench.ParseTreeSource("worktree", "/tmp/x")
	if err != nil || src.Type != workbench.SourceTypeWorktree || src.Id != "/tmp/x" {
		t.Errorf("合法输入解析不符: %+v, err=%v", src, err)
	}
}

func TestWorkbenchCommits(t *testing.T) {
	env := newTestEnv(t)
	repo := env.ws.Join("g1/proj1")

	var p1 struct {
		List    []map[string]any `json:"list"`
		HasMore bool             `json:"hasMore"`
		NextCur int              `json:"nextCursor"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/commits?path="+repo+"&limit=1")), &p1)
	if len(p1.List) != 1 || !p1.HasMore || p1.NextCur != 1 {
		t.Fatalf("第一页不符: len=%d hasMore=%v next=%d", len(p1.List), p1.HasMore, p1.NextCur)
	}
	if _, ok := p1.List[0]["parents"].([]any); !ok {
		t.Errorf("parents 应为数组: %v", p1.List[0]["parents"])
	}

	var p2 struct {
		List    []map[string]any `json:"list"`
		HasMore bool             `json:"hasMore"`
	}
	// fixture 仓库仅 1 个 commit：第二页应为空且无更多
	decodeData(t, getJSON(t, env.url("/api/workbench/commits?path="+repo+"&limit=1&cursor=1")), &p2)
	if len(p2.List) != 0 || p2.HasMore {
		t.Fatalf("第二页应为空: len=%d hasMore=%v", len(p2.List), p2.HasMore)
	}

	if r := getJSON(t, env.url("/api/workbench/commits?path="+repo+"&scope=bogus")); r.Ok {
		t.Error("非法 scope 应报错")
	}
}

func TestWorkbenchStatus(t *testing.T) {
	env := newTestEnv(t)
	repo := env.ws.Join("g1/proj1")

	var got struct {
		Branch string `json:"branch"`
		Sha    string `json:"sha"`
		Dirty  bool   `json:"dirty"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/status?path="+repo)), &got)
	if got.Branch == "" || got.Sha == "" || got.Dirty {
		t.Errorf("干净仓库状态不符: %+v", got)
	}

	// 未提交改动 → dirty
	env.ws.WriteFile("g1/proj1/dirty.txt", []byte("x"))
	var dirty struct {
		Dirty     bool `json:"dirty"`
		Untracked int  `json:"untracked"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/status?path="+repo)), &dirty)
	if !dirty.Dirty || dirty.Untracked != 1 {
		t.Errorf("dirty 状态不符: %+v", dirty)
	}
}

// --- 提案 1012：tree / file 读 + save 写 ---

func TestWorkbenchTreeAndFile(t *testing.T) {
	env := newTestEnv(t)
	repo := env.ws.Join("g1/proj1")
	env.ws.WriteFile("g1/proj1/a.txt", []byte("hello"))
	env.ws.WriteFile("g1/proj1/sub/b.txt", []byte("sub"))
	env.ws.WriteFile("g1/proj1/ignored.log", []byte("x"))
	_ = exec.Command("git", "-C", repo, "add", "-A").Run()
	if err := git.Commit(repo, "workbench test"); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	head := gitHead(t, repo)

	// worktree 源：真实文件树，ignored.log 被 .gitignore 过滤（fixture 项目自身配置忽略 *.log 时才成立，
	// 这里不依赖 fixture 的 ignore 配置，直接验证 commit 源与 worktree 源的一致性）
	var wtTree []struct {
		Name string `json:"name"`
		Dir  bool   `json:"dir"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/tree?path="+repo+"&sourceType=worktree&sourceId="+urlQueryEscape(repo))), &wtTree)
	if len(wtTree) == 0 {
		t.Fatal("worktree 树不应为空")
	}

	// commit 源：ls-tree 根层
	var cTree []struct {
		Name string `json:"name"`
		Dir  bool   `json:"dir"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/tree?path="+repo+"&sourceType=commit&sourceId="+head)), &cTree)
	if len(cTree) < 2 {
		t.Fatalf("commit 树应含 a.txt 与 sub: %+v", cTree)
	}

	// commit 源读文件
	var cFile struct {
		Content string `json:"content"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/file?path="+repo+"&sourceType=commit&sourceId="+head+"&file=a.txt")), &cFile)
	if cFile.Content != "hello" {
		t.Errorf("commit 读文件不符: %q", cFile.Content)
	}

	// worktree 源读文件
	var wFile struct {
		Content string `json:"content"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/file?path="+repo+"&sourceType=worktree&sourceId="+urlQueryEscape(repo)+"&file=sub/b.txt")), &wFile)
	if wFile.Content != "sub" {
		t.Errorf("worktree 读文件不符: %q", wFile.Content)
	}

	// 路径逃逸被拒
	if r := getJSON(t, env.url("/api/workbench/file?path="+repo+"&sourceType=worktree&sourceId="+urlQueryEscape(repo)+"&file=../../etc/hosts")); r.Ok {
		t.Error("路径逃逸应报错")
	}
}

func TestWorkbenchSaveFile(t *testing.T) {
	env := newTestEnv(t)
	repo := env.ws.Join("g1/proj1")
	env.ws.WriteFile("g1/proj1/edit.txt", []byte("old"))

	// 虚拟源保存被拒
	head := gitHead(t, repo)
	if r := putJSON(t, env.url("/api/workbench/file?path="+repo+"&sourceType=commit&sourceId="+head+"&file=edit.txt"), `{"content":"x"}`); r.Ok {
		t.Error("commit 源保存应被拒")
	}

	// worktree 源保存成功且落盘
	if r := putJSON(t, env.url("/api/workbench/file?path="+repo+"&sourceType=worktree&sourceId="+urlQueryEscape(repo)+"&file=edit.txt"), `{"content":"new"}`); !r.Ok {
		t.Fatalf("worktree 保存应成功: %s", r.Message)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, "edit.txt")); string(got) != "new" {
		t.Errorf("落盘内容不符: %q", got)
	}
	// git 工作区应变脏
	st, _ := git.LoadRepoStatus(repo)
	if !st.Dirty {
		t.Error("保存后工作区应为 dirty")
	}
}

func gitHead(t *testing.T, dir string) string {
	t.Helper()
	out, err := git.RunRead(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD 失败: %v", err)
	}
	return strings.TrimSpace(out)
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(s, "/", "%2F")
}

// putJSON 打 PUT + JSON body 请求，断言 envelope 解码成功后返回 envelope。
func putJSON(t *testing.T, url string, body string) envelope {
	t.Helper()
	resp, err := http.NewRequest(http.MethodPut, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("构造 PUT %s 失败: %v", url, err)
	}
	resp.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	httpResp, err := client.Do(resp)
	if err != nil {
		t.Fatalf("PUT %s 失败: %v", url, err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT %s 应为 200, got %d", url, httpResp.StatusCode)
	}
	var env envelope
	if err := json.NewDecoder(httpResp.Body).Decode(&env); err != nil {
		t.Fatalf("PUT %s 响应不是合法 envelope: %v", url, err)
	}
	return env
}
