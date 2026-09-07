package handlers

import (
	"strings"
	"testing"

	"cube/web"
)

// TestUsageRecentPaths record 后 recent-paths 去重取最新、limit 生效、缺省 5。
func TestUsageRecentPaths(t *testing.T) {
	env := newTestEnv(t)

	r := postJSON(t, env.url("/api/usage/record"), `{"project":"/p/a","opener":"vscode"}`)
	if !r.Ok {
		t.Fatalf("record 应成功, message=%q", r.Message)
	}
	postJSON(t, env.url("/api/usage/record"), `{"project":"/p/b"}`)
	postJSON(t, env.url("/api/usage/record"), `{"project":"/p/a"}`) // 同路径去重取最新

	got := getJSON(t, env.url("/api/usage/recent-paths?limit=1"))
	var out web.ListResult[struct {
		Path string `json:"path"`
	}]
	decodeData(t, got, &out)
	if len(out.List) != 1 || out.List[0].Path != "/p/a" {
		t.Fatalf("recent-paths 应去重取最新且 limit 生效: %+v", out.List)
	}
}

// TestUsageRecord_Invalid 非绝对路径 / 空 project 应报中文错误。
func TestUsageRecord_Invalid(t *testing.T) {
	env := newTestEnv(t)

	r := postJSON(t, env.url("/api/usage/record"), `{"project":"relative/path"}`)
	if r.Ok || !strings.Contains(r.Message, "绝对路径") {
		t.Fatalf("相对路径应报中文错误, message=%q", r.Message)
	}
	r = postJSON(t, env.url("/api/usage/record"), `{}`)
	if r.Ok || !strings.Contains(r.Message, "绝对路径") {
		t.Fatalf("空 project 应报中文错误, message=%q", r.Message)
	}
}

// TestUsageRecentPaths_DefaultLimit 缺省 limit=5。
func TestUsageRecentPaths_DefaultLimit(t *testing.T) {
	env := newTestEnv(t)
	for _, p := range []string{"/p/1", "/p/2", "/p/3", "/p/4", "/p/5", "/p/6"} {
		postJSON(t, env.url("/api/usage/record"), `{"project":"`+p+`"}`)
	}
	got := getJSON(t, env.url("/api/usage/recent-paths"))
	var out web.ListResult[struct {
		Path string `json:"path"`
	}]
	decodeData(t, got, &out)
	if len(out.List) != 5 {
		t.Fatalf("缺省应返回 5 条: %+v", out.List)
	}
	if out.List[0].Path != "/p/6" {
		t.Fatalf("最近使用的应排前: %+v", out.List)
	}
}
