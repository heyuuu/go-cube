package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"

	"cube/project"
	"cube/project/gitcache"
)

func TestProjectList(t *testing.T) {
	env := newTestEnv(t)
	var got struct {
		List []struct {
			Name    string          `json:"name"`
			Group   string          `json:"group"`
			Path    string          `json:"path"`
			Tags    []string        `json:"tags"`
			GitInfo *gitcache.Entry `json:"gitInfo"`
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
	decodeData(t, getJSON(t, env.url("/api/project/info?name=g1:proj1")), &got)
	if got.Project == nil || got.Project.Name != "g1:proj1" || got.Project.Group != "g1" {
		t.Errorf("info 应返回 g1:proj1, got %+v", got.Project)
	}

	// 不存在的 name：ok 仍为 true，data.project 为 null（与列表语义一致）
	env2 := getJSON(t, env.url("/api/project/info?name=not-exist"))
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
		t.Errorf("缺必填 name 应非 200, got %d", resp.StatusCode)
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

	// save 新增
	r := postJSON(t, env.url("/api/project/scan-rule/save"), fmt.Sprintf(`{"group":"g3","path":%q,"maxDepth":2}`, root))
	if !r.Ok {
		t.Fatalf("save 应成功, message=%q", r.Message)
	}
	// save 坏数据：中文错误、不落文件
	bad := postJSON(t, env.url("/api/project/scan-rule/save"), `{"group":"","path":"/x","maxDepth":2}`)
	if bad.Ok || !strings.Contains(bad.Message, "group") {
		t.Fatalf("坏数据应报中文错误, ok=%v message=%q", bad.Ok, bad.Message)
	}

	list := getJSON(t, env.url("/api/project/scan-rules"))
	var got struct {
		List []project.ScanRule `json:"list"`
	}
	decodeData(t, list, &got)
	if len(got.List) != 3 { // fixture 预置 g1/g2 + 新增 g3
		t.Fatalf("应返回 3 条规则, got %d: %+v", len(got.List), got.List)
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
