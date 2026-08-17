package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestOpenerList(t *testing.T) {
	env := newTestEnv(t)
	var got struct {
		List []struct {
			Name  string   `json:"name"`
			Cmd   []string `json:"cmd"`
			Roles []string `json:"roles"`
		} `json:"list"`
	}
	decodeData(t, getJSON(t, env.url("/api/opener/list")), &got)

	// broken（缺 cmd）在 Service 构造时被降级跳过，不应出现
	if len(got.List) != 1 {
		t.Fatalf("应只返回 1 个合法 opener, got %d: %+v", len(got.List), got.List)
	}
	op := got.List[0]
	if op.Name != "finder" {
		t.Errorf("name 应为 finder, got %q", op.Name)
	}
	if len(op.Cmd) != 2 || op.Cmd[0] != "/usr/bin/open" || op.Cmd[1] != "$0" {
		t.Errorf("cmd 应保留 $0 占位符原样输出, got %v", op.Cmd)
	}
	if len(op.Roles) != 1 || op.Roles[0] != "open-dir" {
		t.Errorf("roles 应为 [open-dir], got %v", op.Roles)
	}
}

func TestOpenerInfo(t *testing.T) {
	env := newTestEnv(t)

	var got struct {
		Name string `json:"name"`
	}
	decodeData(t, getJSON(t, env.url("/api/opener/info?name=finder")), &got)
	if got.Name != "finder" {
		t.Errorf("info 应返回 finder, got %+v", got)
	}

	// 不存在的 name：data 为 null（与 project info 语义一致）
	env2 := getJSON(t, env.url("/api/opener/info?name=none"))
	var got2 *struct {
		Name string `json:"name"`
	}
	decodeData(t, env2, &got2)
	if got2 != nil {
		t.Errorf("不存在的 opener 应为 null, got %+v", got2)
	}

	// 缺必填 query 被参数校验拒绝
	resp, err := http.Get(env.url("/api/opener/info"))
	if err != nil {
		t.Fatalf("GET 缺参失败: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Errorf("缺必填 name 应非 200, got %d", resp.StatusCode)
	}
}

func TestOpenerOpen(t *testing.T) {
	env := newTestEnv(t)
	md := env.ws.Join("notes/readme.md")
	env.ws.WriteFile("notes/readme.md", []byte("a"))

	post := func(body string) envelope {
		resp, err := http.Post(env.url("/api/opener/open"), "application/json", bytes.NewReader([]byte(body)))
		if err != nil {
			t.Fatalf("POST opener/open 失败: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST opener/open 应为 200, got %d", resp.StatusCode)
		}
		var env2 envelope
		if err := json.NewDecoder(resp.Body).Decode(&env2); err != nil {
			t.Fatalf("md/open 响应解析失败: %v", err)
		}
		return env2
	}

	// 打开目录：finder（open-dir）合法，fakeExecutor 应收到组装后的命令
	env1 := post(`{"path":"` + env.ws.Join("notes") + `","app":"finder"}`)
	if !env1.Ok {
		t.Fatalf("打开目录应成功, message=%q", env1.Message)
	}
	if env.exec.callCount() != 1 || env.exec.lastCall()[0] != "/usr/bin/open" {
		t.Errorf("executor 应收到 finder 命令, calls=%v", env.exec.calls)
	}

	// role 不匹配：finder 只有 open-dir，开文件应报错
	env2 := post(`{"path":"` + md + `","app":"finder"}`)
	if env2.Ok || !strings.Contains(env2.Message, "open-file") {
		t.Errorf("open-dir opener 开文件应报 role 错误, got ok=%v message=%q", env2.Ok, env2.Message)
	}

	// 不存在的 opener
	env3 := post(`{"path":"` + md + `","app":"nope"}`)
	if env3.Ok || !strings.Contains(env3.Message, "未找到指定 app") {
		t.Errorf("未知 opener 应报错, got ok=%v message=%q", env3.Ok, env3.Message)
	}
}
