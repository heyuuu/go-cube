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
		Root          string `json:"root"`
		DefaultBranch string `json:"defaultBranch"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/info?path="+repo)), &got)
	if got.Root != repo {
		t.Errorf("root 应为 %s, got %q", repo, got.Root)
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
		Head    string   `json:"head"`
		Remotes []any    `json:"remotes"`
		Tags    []string `json:"tags"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/refs?path="+repo)), &got)
	if len(got.Locals) == 0 {
		t.Fatal("本地分支不应为空")
	}
	if got.Head == "" {
		t.Fatal("当前分支不应为空")
	}
	// 列表是写方契约：前端选中 ref 时整串作为 TreeSource id，必须是规范全名
	if !strings.HasPrefix(got.Locals[0], "refs/heads/") {
		t.Errorf("locals 应为规范全名: %v", got.Locals)
	}
	if !strings.HasPrefix(got.Head, "refs/heads/") {
		t.Errorf("head 应为规范全名: %q", got.Head)
	}
	// 无 remote/tag 的 fixture：remotes/tags 是 nil 切片，envelope 应序列化为 []
	if got.Remotes == nil || got.Tags == nil {
		t.Errorf("nil 切片应序列化为 [], got remotes=%v tags=%v", got.Remotes, got.Tags)
	}
}

// TestWorkbenchSourceParse 校验 TreeSource 解析纯函数（HTTP 契约的一部分）。
// 合法用例同时断言 String() 往返一致（写方序列化与解析成对）。
func TestWorkbenchSourceParse(t *testing.T) {
	sha40 := strings.Repeat("a1", 20)
	sha64 := strings.Repeat("b2", 32)
	cases := []struct {
		name    string
		in      string
		want    workbench.TreeSource
		wantErr bool
	}{
		{"commit 40 位 sha", "commit://" + sha40, workbench.TreeSource{Type: workbench.SourceTypeCommit, Id: sha40}, false},
		{"commit 64 位 sha（sha256 仓库）", "commit://" + sha64, workbench.TreeSource{Type: workbench.SourceTypeCommit, Id: sha64}, false},
		{"ref 全名-分支", "ref://refs/heads/master", workbench.TreeSource{Type: workbench.SourceTypeRef, Id: "refs/heads/master"}, false},
		{"ref 全名-tag", "ref://refs/tags/v1.0", workbench.TreeSource{Type: workbench.SourceTypeRef, Id: "refs/tags/v1.0"}, false},
		{"ref 全名-远程跟踪", "ref://refs/remotes/origin/dev", workbench.TreeSource{Type: workbench.SourceTypeRef, Id: "refs/remotes/origin/dev"}, false},
		{"ref 全名-开放子树", "ref://refs/pull/123/head", workbench.TreeSource{Type: workbench.SourceTypeRef, Id: "refs/pull/123/head"}, false},
		{"ref 相对路径短名", "ref://heads/master", workbench.TreeSource{Type: workbench.SourceTypeRef, Id: "heads/master"}, false},
		{"ref 短名-HEAD", "ref://HEAD", workbench.TreeSource{Type: workbench.SourceTypeRef, Id: "HEAD"}, false},
		{"ref 短名-远程叶名", "ref://origin/dev", workbench.TreeSource{Type: workbench.SourceTypeRef, Id: "origin/dev"}, false},
		{"worktree 绝对路径", "worktree:///tmp/x", workbench.TreeSource{Type: workbench.SourceTypeWorktree, Id: "/tmp/x"}, false},
		{"worktree 路径含冒号不歧义", "worktree:///tmp/a://b", workbench.TreeSource{Type: workbench.SourceTypeWorktree, Id: "/tmp/a://b"}, false},
		{"缺 scheme", "master", workbench.TreeSource{}, true},
		{"未知 scheme", "branch://main", workbench.TreeSource{}, true},
		{"空 id", "ref://", workbench.TreeSource{}, true},
		{"commit 非 hex", "commit://" + strings.Repeat("z", 40), workbench.TreeSource{}, true},
		{"commit 长度不符", "commit://abc123", workbench.TreeSource{}, true},
		{"rev 表达式-波浪号", "ref://HEAD~2", workbench.TreeSource{}, true},
		{"rev 表达式-upstream", "ref://@{u}", workbench.TreeSource{}, true},
		{"rev 表达式-插入符", "ref://master^", workbench.TreeSource{}, true},
		{"ref 短名含空格", "ref://a b", workbench.TreeSource{}, true},
		{"ref 短名含冒号", "ref://a:b", workbench.TreeSource{}, true},
		{"ref 短名连续点", "ref://master..dev", workbench.TreeSource{}, true},
		{"ref 短名 lock 结尾", "ref://x.lock", workbench.TreeSource{}, true},
		{"ref 短名前导点", "ref://.hidden", workbench.TreeSource{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := workbench.ParseTreeSource(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("应报错: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("不应报错: %v", err)
			}
			if got != tc.want {
				t.Errorf("解析不符: got %+v, want %+v", got, tc.want)
			}
			if got.String() != tc.in {
				t.Errorf("String 往返不一致: got %q, want %q", got.String(), tc.in)
			}
		})
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
	// fixture 仓库恰 1 个 commit，limit=1：整倍边界应精确判定无更多（曾误报 true）
	decodeData(t, getJSON(t, env.url("/api/workbench/commits?path="+repo+"&limit=1")), &p1)
	if len(p1.List) != 1 || p1.HasMore || p1.NextCur != 1 {
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

}

func TestWorkbenchWorktrees(t *testing.T) {
	env := newTestEnv(t)
	repo := env.ws.Join("g1/proj1")

	var got []struct {
		Path      string `json:"path"`
		Head      string `json:"head"`
		Branch    string `json:"branch"`
		Dirty     bool   `json:"dirty"`
		Untracked int    `json:"untracked"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/worktrees?path="+repo)), &got)
	if len(got) != 1 || got[0].Path != repo {
		t.Fatalf("应有且仅有主工作副本: %+v", got)
	}
	if got[0].Branch == "" || got[0].Head == "" || got[0].Dirty {
		t.Errorf("干净副本快照不符: %+v", got[0])
	}

	// 未提交改动 → dirty + untracked 计数（虚拟节点数据源）
	env.ws.WriteFile("g1/proj1/dirty.txt", []byte("x"))
	decodeData(t, getJSON(t, env.url("/api/workbench/worktrees?path="+repo)), &got)
	if len(got) != 1 || !got[0].Dirty || got[0].Untracked != 1 {
		t.Errorf("dirty 副本快照不符: %+v", got)
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

	// worktree 源：全量扁平路径。写一个未跟踪文件与一个被忽略文件验证口径——
	// git 管理的（tracked + 未跟踪未忽略）进清单，被忽略的不进
	env.ws.WriteFile("g1/proj1/untracked-new.txt", []byte("x"))
	env.ws.WriteFile("g1/proj1/ignored-new.log", []byte("x"))
	env.ws.WriteFile("g1/proj1/.gitignore", []byte("*.log\n"))
	var wtTree struct {
		List []string `json:"list"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/tree?path="+repo+"&source="+urlQueryEscape("worktree://"+repo))), &wtTree)
	set := map[string]bool{}
	for _, f := range wtTree.List {
		set[f] = true
	}
	if !set["a.txt"] || !set["untracked-new.txt"] {
		t.Fatalf("清单应含 tracked 与未跟踪未忽略文件: %v", wtTree.List)
	}
	if set["ignored-new.log"] {
		t.Fatalf("被忽略文件不应出现在清单: %v", wtTree.List)
	}

	// commit 源：该提交树的全量文件（扁平路径）
	var cTree struct {
		List []string `json:"list"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/tree?path="+repo+"&source=commit://"+head)), &cTree)
	if len(cTree.List) == 0 {
		t.Fatalf("commit 源清单不应为空")
	}
	for _, f := range cTree.List {
		if f != "a.txt" && f != "sub/b.txt" && f != ".gitignore" && f != "ignored.log" {
			t.Fatalf("commit 源清单含意外条目 %q: %v", f, cTree.List)
		}
	}

	// commit 源读文件
	var cFile struct {
		Content string `json:"content"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/file?path="+repo+"&source=commit://"+head+"&file=a.txt")), &cFile)
	if cFile.Content != "hello" {
		t.Errorf("commit 读文件不符: %q", cFile.Content)
	}

	// worktree 源读文件
	var wFile struct {
		Content string `json:"content"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/file?path="+repo+"&source="+urlQueryEscape("worktree://"+repo)+"&file=sub/b.txt")), &wFile)
	if wFile.Content != "sub" {
		t.Errorf("worktree 读文件不符: %q", wFile.Content)
	}

	// 路径逃逸被拒
	if r := getJSON(t, env.url("/api/workbench/file?path="+repo+"&source="+urlQueryEscape("worktree://"+repo)+"&file=../../etc/hosts")); r.Ok {
		t.Error("路径逃逸应报错")
	}
}

func TestWorkbenchSaveFile(t *testing.T) {
	env := newTestEnv(t)
	repo := env.ws.Join("g1/proj1")
	env.ws.WriteFile("g1/proj1/edit.txt", []byte("old"))

	// 虚拟源保存被拒
	head := gitHead(t, repo)
	if r := postJSON(t, env.url("/api/workbench/file/save"), fmt.Sprintf(`{"path":%q,"source":"commit://%s","file":"edit.txt","content":"x"}`, repo, head)); r.Ok {
		t.Error("commit 源保存应被拒")
	}

	// worktree 源保存成功且落盘
	if r := postJSON(t, env.url("/api/workbench/file/save"), fmt.Sprintf(`{"path":%q,"source":"worktree://%s","file":"edit.txt","content":"new"}`, repo, repo)); !r.Ok {
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

	// 工作区改动：mod 改、del 删、new 增（untracked）、ignored.log 增（ignored）、
	// ren-old 改名 ren-new（内容不变）
	env.ws.WriteFile("g1/proj1/mod.txt", []byte("v2 line1\nv2 line2\n"))
	os.Remove(filepath.Join(repo, "del.txt"))
	env.ws.WriteFile("g1/proj1/new.txt", []byte("new file"))
	env.ws.WriteFile("g1/proj1/ignored.log", []byte("ignored"))
	env.ws.WriteFile("g1/proj1/.gitignore", []byte("*.log\n"))
	_ = exec.Command("git", "-C", repo, "add", ".gitignore").Run()
	_ = exec.Command("git", "-C", repo, "mv", "ren-old.txt", "ren-new.txt").Run()
	if err := git.Commit(repo, "ignore rules"); err != nil {
		t.Fatalf("提交 ignore rules 失败: %v", err)
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
	wt := urlQueryEscape("worktree://" + repo)
	decodeData(t, getJSON(t, env.url("/api/workbench/diff?path="+repo+"&base=commit://"+head+"&current="+wt)), &got)
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
	// ignored 文件恒不出现（showIgnored 参数已随筛选链移除，ignored 不在产品范围）
	if _, ok := byPath["ignored.log/added"]; ok {
		t.Error("ignored 文件不应出现")
	}
}

func TestWorkbenchFileDiff(t *testing.T) {
	env := newTestEnv(t)
	repo, head := setupDiffRepo(t, env)
	wt := urlQueryEscape("worktree://" + repo)

	var got struct {
		Binary bool `json:"binary"`
		Hunks  []struct {
			OldStart int `json:"oldStart"`
			NewStart int `json:"newStart"`
			OldCount int `json:"oldCount"`
			Lines    []struct {
				Kind string `json:"kind"`
				Text string `json:"text"`
			} `json:"lines"`
		} `json:"hunks"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/file-diff?path="+repo+"&base=commit://"+head+"&current="+wt+"&file=mod.txt")), &got)
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
	decodeData(t, getJSON(t, env.url("/api/workbench/file-diff?path="+repo+"&base=commit://"+head+"&current=commit://"+head+"&file=keep.txt")), &same)
	if len(same.Hunks) != 0 {
		t.Errorf("相同文件应无 hunks: %d", len(same.Hunks))
	}

	// left 缺省 = 相对基准（worktree vs HEAD）：结果与显式 left=HEAD 等价
	var noLeft struct {
		Hunks []struct{} `json:"hunks"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/file-diff?path="+repo+"&current="+wt+"&file=mod.txt")), &noLeft)
	decodeData(t, getJSON(t, env.url("/api/workbench/file-diff?path="+repo+"&base=commit://"+head+"&current="+wt+"&file=mod.txt")), &got)
	if len(noLeft.Hunks) != len(got.Hunks) {
		t.Errorf("left 缺省应与显式 left=HEAD 等价: %d vs %d", len(noLeft.Hunks), len(got.Hunks))
	}

	// 单侧缺失的文件（新增 new.txt 只在右侧）：一侧空白一侧全文，全为 add 行，不报错
	decodeData(t, getJSON(t, env.url("/api/workbench/file-diff?path="+repo+"&base=commit://"+head+"&current="+wt+"&file=new.txt")), &got)
	if got.Binary || len(got.Hunks) == 0 {
		t.Fatalf("new.txt 应呈现整体新增的 hunks: binary=%v hunks=%d", got.Binary, len(got.Hunks))
	}
	onlyAdd := true
	for _, h := range got.Hunks {
		for _, l := range h.Lines {
			if l.Kind != "add" {
				onlyAdd = false
			}
		}
	}
	if !onlyAdd {
		t.Error("新增文件的行应全为 add")
	}
	if got.Hunks[0].OldCount != 0 {
		t.Errorf("左侧空白 oldCount 应为 0: %d", got.Hunks[0].OldCount)
	}

	// rename 未改内容（base 的 ren-old → HEAD 的 ren-new）：leftFile 指旧路径时两侧字节相同
	// → 空 hunks「内容一致」；不带 leftFile 时基准侧读不到新路径 → 一侧全文（旧行为，作对照）
	var base string
	if b, err := exec.Command("git", "-C", repo, "rev-parse", head+"^").Output(); err != nil {
		t.Fatalf("取父提交失败: %v", err)
	} else {
		base = strings.TrimSpace(string(b))
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/file-diff?path="+repo+"&base=commit://"+base+"&current="+wt+"&file=ren-new.txt&baseFile=ren-old.txt")), &got)
	if got.Binary || len(got.Hunks) != 0 {
		t.Errorf("未改内容的 rename 应两侧一致（空 hunks）: binary=%v hunks=%d", got.Binary, len(got.Hunks))
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/file-diff?path="+repo+"&base=commit://"+base+"&current="+wt+"&file=ren-new.txt")), &got)
	if len(got.Hunks) == 0 {
		t.Error("不带 leftFile 时基准侧按新路径读不到，应呈现整体新增（对照）")
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
	env.ws.WriteFile("g1/proj1/bdir/x.txt", []byte("b"))

	var got struct {
		List []string `json:"list"`
	}
	decodeData(t, getJSON(t, env.url("/api/workbench/tree?path="+repo+"&source="+urlQueryEscape("worktree://"+repo))), &got)
	set := map[string]bool{}
	for _, f := range got.List {
		set[f] = true
	}
	for _, want := range []string{"afile.txt", "bdir/x.txt"} {
		if !set[want] {
			t.Fatalf("全量清单应含 %q: %v", want, got.List)
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
	decodeData(t, getJSON(t, env.url("/api/workbench/changes?path="+repo+"&source="+urlQueryEscape("worktree://"+repo))), &wt)
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
	decodeData(t, getJSON(t, env.url("/api/workbench/changes?path="+repo+"&source=commit://"+head)), &c)
	if len(c.List) != 2 {
		t.Errorf("commit 差异应含 2 个文件: %+v", c.List)
	}
}
