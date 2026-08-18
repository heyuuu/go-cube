package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"cube/util/git"
	"cube/workbench"

	"github.com/coder/websocket"
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
	if r := postJSON(t, env.url("/api/workbench/file/save"), fmt.Sprintf(`{"path":%q,"sourceType":"commit","sourceId":%q,"file":"edit.txt","content":"x"}`, repo, head)); r.Ok {
		t.Error("commit 源保存应被拒")
	}

	// worktree 源保存成功且落盘
	if r := postJSON(t, env.url("/api/workbench/file/save"), fmt.Sprintf(`{"path":%q,"sourceType":"worktree","sourceId":%q,"file":"edit.txt","content":"new"}`, repo, repo)); !r.Ok {
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
	sha, err := git.HeadSha(dir)
	if err != nil {
		t.Fatalf("读取 HEAD 失败: %v", err)
	}
	return sha
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(s, "/", "%2F")
}

// postJSON 打 POST + JSON body 请求，断言 envelope 解码成功后返回 envelope。
func postJSON(t *testing.T, url string, body string) envelope {
	t.Helper()
	resp, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("构造 POST %s 失败: %v", url, err)
	}
	resp.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	httpResp, err := client.Do(resp)
	if err != nil {
		t.Fatalf("POST %s 失败: %v", url, err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s 应为 200, got %d", url, httpResp.StatusCode)
	}
	var env envelope
	if err := json.NewDecoder(httpResp.Body).Decode(&env); err != nil {
		t.Fatalf("POST %s 响应不是合法 envelope: %v", url, err)
	}
	return env
}

// --- 提案 1013：diff / file-diff ---

func setupDiffRepo(t *testing.T, env *testEnv) (repo, head string) {
	t.Helper()
	repo = env.ws.Join("g1/proj1")
	env.ws.WriteFile("g1/proj1/keep.txt", []byte("same"))
	env.ws.WriteFile("g1/proj1/mod.txt", []byte("v1"))
	env.ws.WriteFile("g1/proj1/del.txt", []byte("bye"))
	env.ws.WriteFile("g1/proj1/ren-old.txt", []byte("ren"))
	_ = exec.Command("git", "-C", repo, "add", "-A").Run()
	if err := git.Commit(repo, "base"); err != nil {
		t.Fatalf("提交 base 失败: %v", err)
	}
	head = gitHead(t, repo)

	// 工作区改动：mod 改、del 删、new 增（untracked）、ignored.log 增（ignored）
	env.ws.WriteFile("g1/proj1/mod.txt", []byte("v2 line1\nv2 line2\n"))
	os.Remove(filepath.Join(repo, "del.txt"))
	env.ws.WriteFile("g1/proj1/new.txt", []byte("new file"))
	env.ws.WriteFile("g1/proj1/ignored.log", []byte("ignored"))
	env.ws.WriteFile("g1/proj1/.gitignore", []byte("*.log\n"))
	_ = exec.Command("git", "-C", repo, "add", ".gitignore").Run()
	if err := git.Commit(repo, "ignore rules"); err != nil {
		t.Fatalf("提交 ignore 失败: %v", err)
	}
	return repo, gitHead(t, repo)
}

func TestWorkbenchDiffGit(t *testing.T) {
	env := newTestEnv(t)
	repo, head := setupDiffRepo(t, env)

	// worktree(工作区) vs commit(head)：fs 模式
	var got struct {
		Mode string `json:"mode"`
		List []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
		} `json:"list"`
	}
	wt := urlQueryEscape(repo)
	decodeData(t, getJSON(t, env.url("/api/workbench/diff?path="+repo+"&leftType=commit&leftId="+head+"&rightType=worktree&rightId="+wt)), &got)
	if got.Mode != "fs" {
		t.Fatalf("含 worktree 源应为 fs 模式, got %q", got.Mode)
	}
	byPath := map[string]string{}
	for _, e := range got.List {
		byPath[e.Path+"/"+e.Status] = ""
	}
	for _, want := range []string{"mod.txt/modified", "del.txt/deleted", "new.txt/added"} {
		if _, ok := byPath[want]; !ok {
			t.Errorf("缺少 %s（got %v）", want, keysOf(byPath))
		}
	}
	// ignored.log 默认被过滤
	if _, ok := byPath["ignored.log/added"]; ok {
		t.Error("ignored 文件默认不应出现")
	}

	// showIgnored=true：ignored.log 出现
	decodeData(t, getJSON(t, env.url("/api/workbench/diff?path="+repo+"&leftType=commit&leftId="+head+"&rightType=worktree&rightId="+wt+"&showIgnored=true")), &got)
	found := false
	for _, e := range got.List {
		if e.Path == "ignored.log" {
			found = true
		}
	}
	if !found {
		t.Error("showIgnored=true 时 ignored.log 应出现")
	}

	// statusFilter 过滤
	decodeData(t, getJSON(t, env.url("/api/workbench/diff?path="+repo+"&leftType=commit&leftId="+head+"&rightType=worktree&rightId="+wt+"&statusFilter=modified")), &got)
	if len(got.List) != 1 || got.List[0].Path != "mod.txt" {
		t.Errorf("statusFilter=modified 应只剩 mod.txt: %+v", got.List)
	}
}

func TestWorkbenchFileDiff(t *testing.T) {
	env := newTestEnv(t)
	repo, head := setupDiffRepo(t, env)
	wt := urlQueryEscape(repo)

	var got struct {
		Binary bool `json:"binary"`
		Hunks  []struct {
			OldStart int `json:"oldStart"`
			NewStart int `json:"newStart"`
			Lines    []struct {
				Kind string `json:"kind"`
				Text string `json:"text"`
			} `json:"lines"`
		} `json:"hunks"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/file-diff?path="+repo+"&leftType=commit&leftId="+head+"&rightType=worktree&rightId="+wt+"&file=mod.txt")), &got)
	if got.Binary || len(got.Hunks) == 0 {
		t.Fatalf("mod.txt 应有 diff hunks: binary=%v hunks=%d", got.Binary, len(got.Hunks))
	}
	kinds := map[string]int{}
	for _, h := range got.Hunks {
		for _, l := range h.Lines {
			kinds[l.Kind]++
		}
	}
	if kinds["del"] == 0 || kinds["add"] == 0 {
		t.Errorf("应同时含 del 与 add 行: %v", kinds)
	}

	// 相同文件 → 空 hunks
	var same struct {
		Hunks []struct{} `json:"hunks"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/file-diff?path="+repo+"&leftType=commit&leftId="+head+"&rightType=commit&rightId="+head+"&file=keep.txt")), &same)
	if len(same.Hunks) != 0 {
		t.Errorf("相同文件应无 hunks: %d", len(same.Hunks))
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// --- 提案 1014：PTY WebSocket 链路（连接-输入-收输出-退出） ---

func TestWorkbenchPty(t *testing.T) {
	env := newTestEnv(t)
	dir := env.ws.Dir

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(env.ts.URL, "http") + "/api/workbench/pty?path=" + urlQueryEscape(dir) + "&cols=100&rows=30"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws 连接失败: %v", err)
	}
	defer conn.CloseNow()

	// 输入一条命令并执行 exit
	send := func(msg string) {
		t.Helper()
		if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"input","data":`+strconv.Quote(msg)+`}`)); err != nil {
			t.Fatalf("发送失败: %v", err)
		}
	}
	send("echo cube_pty_marker\r")
	send("exit\r")

	// 持续读帧，直到看到标记或 exit 帧或超时
	gotMarker, gotExit := false, false
	deadline := time.Now().Add(10 * time.Second)
	for (!gotMarker || !gotExit) && time.Now().Before(deadline) {
		rctx, rcancel := context.WithTimeout(ctx, 3*time.Second)
		_, data, err := conn.Read(rctx)
		rcancel()
		if err != nil {
			break
		}
		var msg struct {
			Type string `json:"type"`
			Data string `json:"data"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		switch msg.Type {
		case "output":
			if strings.Contains(msg.Data, "cube_pty_marker") {
				gotMarker = true
			}
		case "exit":
			gotExit = true
		}
	}
	if !gotMarker {
		t.Error("应在输出中看到 echo 标记")
	}
	if !gotExit {
		t.Error("子进程退出后应收 exit 帧")
	}
}

// --- 排序 + 差异模式 ---

func TestWorkbenchTreeSort(t *testing.T) {
	env := newTestEnv(t)
	repo := env.ws.Join("g1/proj1")
	env.ws.Mkdir("g1/proj1/zdir")
	env.ws.WriteFile("g1/proj1/zdir/x.txt", []byte("x"))
	env.ws.WriteFile("g1/proj1/afile.txt", []byte("a"))
	env.ws.WriteFile("g1/proj1/bdir-nofile", []byte("b"))

	var got []struct {
		Name string `json:"name"`
		Dir  bool   `json:"dir"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/tree?path="+repo+"&sourceType=worktree&sourceId="+urlQueryEscape(repo))), &got)
	var names []string
	for _, e := range got {
		names = append(names, fmt.Sprintf("%v:%s", e.Dir, e.Name))
	}
	want := []string{"true:zdir", "false:afile.txt", "false:bdir-nofile"}
	if len(names) != len(want) {
		t.Fatalf("条目数不符: %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("排序不符: got %v, want %v", names, want)
			break
		}
	}
}

func TestWorkbenchChanges(t *testing.T) {
	env := newTestEnv(t)
	repo := env.ws.Join("g1/proj1")
	env.ws.WriteFile("g1/proj1/keep.txt", []byte("same"))
	env.ws.WriteFile("g1/proj1/old.txt", []byte("v1"))
	_ = exec.Command("git", "-C", repo, "add", "-A").Run()
	_ = git.Commit(repo, "base")
	head := gitHead(t, repo)

	// 工作区改动：mod + 新增（untracked）
	env.ws.WriteFile("g1/proj1/old.txt", []byte("v2"))
	env.ws.WriteFile("g1/proj1/new.txt", []byte("n"))

	var wt struct {
		List []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
		} `json:"list"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/changes?path="+repo+"&sourceType=worktree&sourceId="+urlQueryEscape(repo))), &wt)
	byPath := map[string]string{}
	for _, e := range wt.List {
		byPath[e.Path] = e.Status
	}
	if byPath["old.txt"] != "modified" || byPath["new.txt"] != "added" {
		t.Errorf("worktree 差异不符: %v", byPath)
	}
	if _, ok := byPath["keep.txt"]; ok {
		t.Error("未变更文件不应出现")
	}

	// commit 源：head 与父提交（fixture 根提交）比 → 两个文件都是 added
	var c struct {
		List []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
		} `json:"list"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/changes?path="+repo+"&sourceType=commit&sourceId="+head)), &c)
	if len(c.List) != 2 {
		t.Errorf("commit 差异应含 2 个文件: %+v", c.List)
	}
}
