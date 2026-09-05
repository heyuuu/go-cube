package handlers

import (
	"strings"
	"testing"

	"cube/forge"
	"cube/web"
)

// TestForgeListEmpty 空配置时 list 返回空数组（nil 切片序列化为 []）。
func TestForgeListEmpty(t *testing.T) {
	env := newTestEnv(t)
	got := getJSON(t, env.url("/api/forge/list"))
	var out web.ListResult[forge.Forge]
	decodeData(t, got, &out)
	if out.List == nil || len(out.List) != 0 {
		t.Fatalf("空配置应返回 []（nil 序列化）, got %#v", out.List)
	}
}

// TestForgeWrite forge 的 save/delete 出口契约（含坏数据中文错误）。
func TestForgeWrite(t *testing.T) {
	env := newTestEnv(t)

	// save 新增（带 icon，host 大小写归一化）
	r := postJSON(t, env.url("/api/forge/save"), `{"host":"GitHub.COM","kind":"github","icon":{"type":"lucide","value":"github"}}`)
	if !r.Ok {
		t.Fatalf("save 应成功, message=%q", r.Message)
	}
	got := getJSON(t, env.url("/api/forge/list"))
	var out web.ListResult[forge.Forge]
	decodeData(t, got, &out)
	if len(out.List) != 1 || out.List[0].Host != "github.com" || out.List[0].Kind != "github" {
		t.Fatalf("保存后 forge 字段不符: %+v", out.List)
	}
	if out.List[0].Icon == nil || out.List[0].Icon.Value != "github" {
		t.Fatalf("icon 应保存: %+v", out.List[0].Icon)
	}

	// 同 host 替换（改 kind）
	r = postJSON(t, env.url("/api/forge/save"), `{"host":"github.com","kind":"generic"}`)
	if !r.Ok {
		t.Fatalf("替换应成功, message=%q", r.Message)
	}
	got = getJSON(t, env.url("/api/forge/list"))
	decodeData(t, got, &out)
	if len(out.List) != 1 || out.List[0].Kind != "generic" {
		t.Fatalf("同 host 应替换而非追加: %+v", out.List)
	}

	// save 坏数据：kind 未知 / host 带 URL，中文错误、不落文件
	badKind := postJSON(t, env.url("/api/forge/save"), `{"host":"gitlab.com","kind":"gitlab"}`)
	if badKind.Ok || !strings.Contains(badKind.Message, "kind") {
		t.Fatalf("坏 kind 应报中文错误, message=%q", badKind.Message)
	}
	badHost := postJSON(t, env.url("/api/forge/save"), `{"host":"https://github.com/x","kind":"github"}`)
	if badHost.Ok || !strings.Contains(badHost.Message, "纯域名") {
		t.Fatalf("坏 host 应报中文错误, message=%q", badHost.Message)
	}
	got = getJSON(t, env.url("/api/forge/list"))
	decodeData(t, got, &out)
	if len(out.List) != 1 {
		t.Fatalf("坏数据不应落文件, got %+v", out.List)
	}

	// delete 按 host；再删报中文错误
	if r := postJSON(t, env.url("/api/forge/delete"), `{"host":"github.com"}`); !r.Ok {
		t.Fatalf("delete 应成功, message=%q", r.Message)
	}
	r2 := postJSON(t, env.url("/api/forge/delete"), `{"host":"github.com"}`)
	if r2.Ok || !strings.Contains(r2.Message, "未找到") {
		t.Fatalf("删不存在应报中文错误, message=%q", r2.Message)
	}
}
