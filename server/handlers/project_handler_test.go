package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"cube/project"
	"cube/project/projcache"
)

func TestProjectList(t *testing.T) {
	env := newTestEnv(t)
	var got struct {
		List []struct {
			Name    string           `json:"name"`
			Group   string           `json:"group"`
			Path    string           `json:"path"`
			Tags    []string         `json:"tags"`
			GitInfo *projcache.Entry `json:"gitInfo"`
		} `json:"list"`
	}
	decodeData(t, getJSON(t, env.url("/api/project/list")), &got)

	if len(got.List) != 2 {
		t.Fatalf("应扫描到 2 个项目, got %d: %+v", len(got.List), got.List)
	}
	byName := map[string]int{}
	for i, p := range got.List {
		byName[p.Name] = i
	}
	p1, ok := byName["g1:proj1"]
	if !ok {
		t.Fatalf("应包含 g1:proj1, got %+v", got.List)
	}
	if got.List[p1].Group != "g1" || got.List[p1].Path != env.proj1Path() {
		t.Errorf("g1:proj1 字段不符: %+v", got.List[p1])
	}
	// nil tags 序列化为 []（nilSliceJSONFormat 契约），gitInfo 未采集为 null
	if got.List[p1].Tags == nil {
		t.Error("nil tags 应序列化为 [] 而非 null")
	}
	if got.List[p1].GitInfo != nil {
		t.Errorf("未采集的 gitInfo 应为 null, got %+v", got.List[p1].GitInfo)
	}
	p2 := got.List[byName["g2:proj2"]]
	if len(p2.Tags) != 1 || p2.Tags[0] != "godot" {
		t.Errorf("g2:proj2 应带 godot tag, got %v", p2.Tags)
	}
}

func TestProjectInfo(t *testing.T) {
	env := newTestEnv(t)

	var got struct {
		Project *struct {
			Name  string `json:"name"`
			Group string `json:"group"`
		} `json:"project"`
	}
	decodeData(t, getJSON(t, env.url("/api/project/info?path="+env.proj1Path())), &got)
	if got.Project == nil || got.Project.Name != "g1:proj1" || got.Project.Group != "g1" {
		t.Errorf("info 应返回 g1:proj1, got %+v", got.Project)
	}

	// 不存在的 path：ok 仍为 true，data.project 为 null（与列表语义一致）
	env2 := getJSON(t, env.url("/api/project/info?path=/not/exist"))
	var got2 struct {
		Project *struct {
			Name string `json:"name"`
		} `json:"project"`
	}
	decodeData(t, env2, &got2)
	if got2.Project != nil {
		t.Errorf("不存在的项目应为 null, got %+v", got2.Project)
	}

	// 缺必填 query：huma 参数校验拒绝（422），不应进 handler
	resp, err := http.Get(env.url("/api/project/info"))
	if err != nil {
		t.Fatalf("GET 缺参失败: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Errorf("缺必填 path 应非 200, got %d", resp.StatusCode)
	}
}

func TestProjectScanRules(t *testing.T) {
	env := newTestEnv(t)
	var got struct {
		List []struct {
			Group    string `json:"group"`
			Path     string `json:"path"`
			MaxDepth int    `json:"maxDepth"`
		} `json:"list"`
	}
	decodeData(t, getJSON(t, env.url("/api/project/scan-rules")), &got)
	if len(got.List) != 2 {
		t.Fatalf("应返回 2 条 scan 规则, got %d", len(got.List))
	}
	// 配置里的路径在领域层已展开为绝对路径
	if got.List[0].Path != env.ws.Join("g1") || got.List[0].Group != "g1" || got.List[0].MaxDepth != 1 {
		t.Errorf("scan 规则字段不符: %+v", got.List[0])
	}
}

func TestProjectCloneRules(t *testing.T) {
	env := newTestEnv(t)
	var got struct {
		List []struct {
			RepoHost   string `json:"repoHost"`
			RepoPrefix string `json:"repoPrefix"`
			LocalPath  string `json:"localPath"`
		} `json:"list"`
	}
	decodeData(t, getJSON(t, env.url("/api/project/clone-rules")), &got)
	if len(got.List) != 1 {
		t.Fatalf("应返回 1 条 clone 规则, got %d", len(got.List))
	}
	if got.List[0].RepoHost != "github.com" || got.List[0].LocalPath != env.ws.Join("repo") {
		t.Errorf("clone 规则字段不符: %+v", got.List[0])
	}
}

// TestScanRuleWrite scan-rule 的 save/delete/reorder 出口契约（含坏数据中文错误）。
func TestScanRuleWrite(t *testing.T) {
	env := newTestEnv(t)
	root := env.ws.Mkdir("g3")

	// save 新增（带 icon）
	r := postJSON(t, env.url("/api/project/scan-rule/save"), fmt.Sprintf(`{"group":"g3","path":%q,"maxDepth":2,"icon":{"type":"lucide","value":"folder-git-2"}}`, root))
	if !r.Ok {
		t.Fatalf("save 应成功, message=%q", r.Message)
	}
	// save 坏数据：中文错误、不落文件
	bad := postJSON(t, env.url("/api/project/scan-rule/save"), `{"group":"","path":"/x","maxDepth":2}`)
	if bad.Ok || !strings.Contains(bad.Message, "group") {
		t.Fatalf("坏数据应报中文错误, ok=%v message=%q", bad.Ok, bad.Message)
	}
	// save 坏 icon：中文错误
	badIcon := postJSON(t, env.url("/api/project/scan-rule/save"), fmt.Sprintf(`{"group":"g4","path":%q,"maxDepth":2,"icon":{"type":"svg","value":"x"}}`, root))
	if badIcon.Ok || !strings.Contains(badIcon.Message, "icon type") {
		t.Fatalf("坏 icon 应报中文错误, message=%q", badIcon.Message)
	}

	list := getJSON(t, env.url("/api/project/scan-rules"))
	var got struct {
		List []project.ScanRule `json:"list"`
	}
	decodeData(t, list, &got)
	if len(got.List) != 3 { // fixture 预置 g1/g2 + 新增 g3
		t.Fatalf("应返回 3 条规则, got %d: %+v", len(got.List), got.List)
	}
	for _, r := range got.List { // icon 落盘并回读（g3 带，g1/g2 未带为 nil）
		if r.Group == "g3" && (r.Icon == nil || r.Icon.Value != "folder-git-2") {
			t.Fatalf("g3 icon 未回读: %+v", r.Icon)
		}
	}

	// reorder：把 g3 提到最前
	paths := make([]string, 0, len(got.List))
	for _, r := range got.List {
		paths = append(paths, r.Path)
	}
	sort.Strings(paths)
	ordered := append([]string{root}, paths[0:len(paths)-1]...)
	body, _ := json.Marshal(map[string]any{"paths": ordered})
	if r := postJSON(t, env.url("/api/project/scan-rule/reorder"), string(body)); !r.Ok {
		t.Fatalf("reorder 应成功, message=%q", r.Message)
	}
	decodeData(t, getJSON(t, env.url("/api/project/scan-rules")), &got)
	if got.List[0].Group != "g3" {
		t.Fatalf("reorder 后 g3 应在最前: %+v", got.List)
	}

	// delete
	if r := postJSON(t, env.url("/api/project/scan-rule/delete"), fmt.Sprintf(`{"path":%q}`, root)); !r.Ok {
		t.Fatalf("delete 应成功, message=%q", r.Message)
	}
	decodeData(t, getJSON(t, env.url("/api/project/scan-rules")), &got)
	if len(got.List) != 2 {
		t.Fatalf("删除后应剩 2 条, got %d", len(got.List))
	}
	// delete 不存在：中文错误
	r2 := postJSON(t, env.url("/api/project/scan-rule/delete"), `{"path":"/no/such"}`)
	if r2.Ok || !strings.Contains(r2.Message, "未找到") {
		t.Fatalf("删除不存在应报中文错误, message=%q", r2.Message)
	}
}

// TestCloneRuleWrite clone-rule 的 save/delete/reorder 出口契约。
func TestCloneRuleWrite(t *testing.T) {
	env := newTestEnv(t)

	// save 新增（prefix 须以 / 开头）
	if r := postJSON(t, env.url("/api/project/clone-rule/save"), fmt.Sprintf(`{"repoHost":"gitee.com","repoPrefix":"/heyuuu","localPath":%q}`, env.ws.Dir)); !r.Ok {
		t.Fatalf("save 应成功, message=%q", r.Message)
	}
	// save 坏数据
	bad := postJSON(t, env.url("/api/project/clone-rule/save"), `{"repoHost":"gitee.com","repoPrefix":"bad","localPath":"/x"}`)
	if bad.Ok || !strings.Contains(bad.Message, "repoPrefix") {
		t.Fatalf("坏数据应报中文错误, ok=%v message=%q", bad.Ok, bad.Message)
	}

	var got struct {
		List []project.CloneRule `json:"list"`
	}
	decodeData(t, getJSON(t, env.url("/api/project/clone-rules")), &got)
	if len(got.List) != 2 { // fixture 预置 github.com + 新增 gitee
		t.Fatalf("应返回 2 条规则, got %d: %+v", len(got.List), got.List)
	}

	// reorder：gitee 提前
	if r := postJSON(t, env.url("/api/project/clone-rule/reorder"), `{"rules":[{"repoHost":"gitee.com","repoPrefix":"/heyuuu"},{"repoHost":"github.com","repoPrefix":""}]}`); !r.Ok {
		t.Fatalf("reorder 应成功, message=%q", r.Message)
	}
	decodeData(t, getJSON(t, env.url("/api/project/clone-rules")), &got)
	if got.List[0].RepoHost != "gitee.com" {
		t.Fatalf("reorder 后 gitee 应在最前: %+v", got.List)
	}

	// delete
	if r := postJSON(t, env.url("/api/project/clone-rule/delete"), `{"repoHost":"gitee.com","repoPrefix":"/heyuuu"}`); !r.Ok {
		t.Fatalf("delete 应成功, message=%q", r.Message)
	}
	decodeData(t, getJSON(t, env.url("/api/project/clone-rules")), &got)
	if len(got.List) != 1 {
		t.Fatalf("删除后应剩 1 条, got %d", len(got.List))
	}
	// delete 不存在
	r2 := postJSON(t, env.url("/api/project/clone-rule/delete"), `{"repoHost":"nope.com","repoPrefix":""}`)
	if r2.Ok || !strings.Contains(r2.Message, "未找到") {
		t.Fatalf("删除不存在应报中文错误, message=%q", r2.Message)
	}
}

func TestProjectOpen(t *testing.T) {
	env := newTestEnv(t)

	post := func(body string) envelope {
		resp, err := http.Post(env.url("/api/project/open"), "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST project/open 失败: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST project/open 应为 200, got %d", resp.StatusCode)
		}
		var env2 envelope
		if err := json.NewDecoder(resp.Body).Decode(&env2); err != nil {
			t.Fatalf("project/open 响应解析失败: %v", err)
		}
		return env2
	}

	// 打开项目成功：fakeExecutor 收到组装后的命令
	env1 := post(fmt.Sprintf(`{"path":%q,"opener":"finder"}`, env.proj1Path()))
	if !env1.Ok {
		t.Fatalf("打开项目应成功, message=%q", env1.Message)
	}
	if env.exec.callCount() != 1 || env.exec.lastCall()[0] != "/usr/bin/open" {
		t.Errorf("executor 应收到 finder 命令, calls=%v", env.exec.calls)
	}

	// 非收录项目路径
	env2 := post(`{"path":"/nonexistent/proj","opener":"finder"}`)
	if env2.Ok || !strings.Contains(env2.Message, "未找到指定项目") {
		t.Errorf("非收录路径应报错, got ok=%v message=%q", env2.Ok, env2.Message)
	}

	// 不存在的 opener
	env3 := post(fmt.Sprintf(`{"path":%q,"opener":"nope"}`, env.proj1Path()))
	if env3.Ok || !strings.Contains(env3.Message, "未找到指定 opener") {
		t.Errorf("未知 opener 应报错, got ok=%v message=%q", env3.Ok, env3.Message)
	}
}

func TestProjectList_LastUsedAt(t *testing.T) {
	env := newTestEnv(t)

	// 打开 proj2（原序列第二位），应产生 usage 记录
	body := fmt.Sprintf(`{"path":%q,"opener":"finder"}`, env.ws.Join("g2", "proj2"))
	resp, err := http.Post(env.url("/api/project/open"), "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST project/open 失败: %v", err)
	}
	resp.Body.Close()

	var got struct {
		List []struct {
			Name       string     `json:"name"`
			LastUsedAt *time.Time `json:"lastUsedAt"`
		} `json:"list"`
	}
	decodeData(t, getJSON(t, env.url("/api/project/list")), &got)

	if len(got.List) != 2 {
		t.Fatalf("应扫描到 2 个项目, got %d", len(got.List))
	}
	// list 保持原始扫描序（排序是前端视图偏好），只补 lastUsedAt：proj2 有、proj1 无
	if got.List[0].Name != "g1:proj1" || got.List[1].Name != "g2:proj2" {
		t.Fatalf("list 应保持扫描原序, got %v", got.List)
	}
	if got.List[1].LastUsedAt == nil {
		t.Error("打开过的 g2:proj2 应带 lastUsedAt")
	}
	if got.List[0].LastUsedAt != nil {
		t.Errorf("未打开过的 g1:proj1 不应有 lastUsedAt, got %v", got.List[0])
	}
}

// TestWorkspaceGetSave workspace/get 三块信息（生效/声明/候选）+ save 全链路（写盘 + 即时重采集）。
func TestWorkspaceGetSave(t *testing.T) {
	env := newTestEnv(t)
	repo := env.proj1Path()

	// 建子目录 + pnpm 声明：未写 cube.json 时探测生效
	env.ws.Mkdir("g1/proj1/apps/web")
	if err := os.WriteFile(env.ws.Join("g1", "proj1", "pnpm-workspace.yaml"), []byte("packages:\n  - 'apps/*'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := env.projSvc.Refresh(); err != nil {
		t.Fatal(err)
	}

	var state struct {
		Effective   []struct{ Name, Path string } `json:"effective"`
		DeclaredSet bool                          `json:"declaredSet"`
		Detected    []struct{ Name, Path string } `json:"detected"`
	}
	decodeData(t, getJSON(t, env.url("/api/project/workspace/get?path="+url.QueryEscape(repo))), &state)
	if state.DeclaredSet {
		t.Fatal("未写 cube.json 时 declaredSet 应为 false")
	}
	if len(state.Effective) != 1 || state.Effective[0].Path != "apps/web" {
		t.Fatalf("探测应生效于生效清单, got %+v", state.Effective)
	}
	if len(state.Detected) != 1 {
		t.Fatalf("探测候选应可得, got %+v", state.Detected)
	}

	// save 固化显式声明 → get 反映 declaredSet，且 effective 即时刷新（不等 TTL）
	body := fmt.Sprintf(`{"path":%q,"workspaces":[{"name":"前端","path":"apps/web"}]}`, repo)
	resp, err := http.Post(env.url("/api/project/workspace/save"), "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var saved envelope
	if err := json.NewDecoder(resp.Body).Decode(&saved); err != nil || !saved.Ok {
		t.Fatalf("save 应成功: err=%v env=%+v", err, saved)
	}

	decodeData(t, getJSON(t, env.url("/api/project/workspace/get?path="+url.QueryEscape(repo))), &state)
	if !state.DeclaredSet {
		t.Fatal("save 后 declaredSet 应为 true")
	}
	if len(state.Effective) != 1 || state.Effective[0].Name != "前端" {
		t.Fatalf("save 后 effective 应即时反映显式声明, got %+v", state.Effective)
	}

	// 坏条目整体拒绝（写侧严格语义，区别于采集侧静默跳过）
	bad := fmt.Sprintf(`{"path":%q,"workspaces":[{"name":"逃逸","path":"../outside"}]}`, repo)
	resp2, err := http.Post(env.url("/api/project/workspace/save"), "application/json", strings.NewReader(bad))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var rejected envelope
	if err := json.NewDecoder(resp2.Body).Decode(&rejected); err != nil {
		t.Fatal(err)
	}
	if rejected.Ok {
		t.Fatal("坏条目应整体拒绝")
	}
}
