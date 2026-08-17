package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

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

func TestProjectOpen(t *testing.T) {
	env := newTestEnv(t)

	post := func(body string) envelope {
		resp, err := http.Post(env.url("/api/project/open"), "application/json", bytes.NewReader([]byte(body)))
		if err != nil {
			t.Fatalf("POST open 失败: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST open 应为 200, got %d", resp.StatusCode)
		}
		var env2 envelope
		if err := json.NewDecoder(resp.Body).Decode(&env2); err != nil {
			t.Fatalf("open 响应解析失败: %v", err)
		}
		return env2
	}

	// 正常打开：executor 收到组装完成的命令（$0 槽位替换为项目路径）
	env1 := post(`{"path":"` + env.proj1Path() + `","app":"finder"}`)
	if !env1.Ok {
		t.Fatalf("open 应成功, message=%q", env1.Message)
	}
	if env.exec.callCount() != 1 {
		t.Fatalf("executor 应被调 1 次, got %d", env.exec.callCount())
	}
	call := env.exec.lastCall()
	if len(call) != 2 || call[0] != "/usr/bin/open" || call[1] != env.proj1Path() {
		t.Errorf("命令组装不符: %v", call)
	}

	// 未配置的 opener：ok=false + 中文错误信息
	env2 := post(`{"path":"` + env.proj1Path() + `","app":"nope"}`)
	if env2.Ok {
		t.Error("未配置 opener 应失败")
	}
	if !contains(env2.Message, "未找到指定 app") {
		t.Errorf("错误信息应含「未找到指定 app」, got %q", env2.Message)
	}

	// 不存在的项目路径
	env3 := post(`{"path":"/not/exist","app":"finder"}`)
	if env3.Ok {
		t.Error("不存在的项目路径应失败")
	}
	if !contains(env3.Message, "未找到指定项目") {
		t.Errorf("错误信息应含「未找到指定项目」, got %q", env3.Message)
	}
	if env.exec.callCount() != 1 {
		t.Errorf("失败路径不应触发执行, got %d 次", env.exec.callCount())
	}
}
