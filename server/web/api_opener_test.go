package web

import (
	"net/http"
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
