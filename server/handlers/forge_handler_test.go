package handlers

import (
	"encoding/json"
	"net/http"
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

// TestForgeReorder reorder 出口契约：重排生效 + 未知 host 中文错误。
func TestForgeReorder(t *testing.T) {
	env := newTestEnv(t)
	postJSON(t, env.url("/api/forge/save"), `{"host":"github.com","kind":"github"}`)
	postJSON(t, env.url("/api/forge/save"), `{"host":"gitee.com","kind":"gitee"}`)

	if r := postJSON(t, env.url("/api/forge/reorder"), `{"hosts":["gitee.com","github.com"]}`); !r.Ok {
		t.Fatalf("reorder 应成功, message=%q", r.Message)
	}
	got := getJSON(t, env.url("/api/forge/list"))
	var out web.ListResult[forge.Forge]
	decodeData(t, got, &out)
	if len(out.List) != 2 || out.List[0].Host != "gitee.com" {
		t.Fatalf("重排后顺序不符: %+v", out.List)
	}

	bad := postJSON(t, env.url("/api/forge/reorder"), `{"hosts":["unknown.com"]}`)
	if bad.Ok || !strings.Contains(bad.Message, "未找到") {
		t.Fatalf("未知 host 应报中文错误, message=%q", bad.Message)
	}
}

// TestForgeAccountApi account 出口契约：保存（token 打码返回）/掩码沿用/删除/坏数据中文错误。
func TestForgeAccountApi(t *testing.T) {
	env := newTestEnv(t)
	postJSON(t, env.url("/api/forge/save"), `{"host":"github.com","kind":"github"}`)

	r := postJSON(t, env.url("/api/forge/account/save"), `{"forgeHost":"github.com","username":"heyuuu","token":"secret"}`)
	if !r.Ok {
		t.Fatalf("保存应成功: %s", r.Message)
	}
	var accounts web.ListResult[forge.Account]
	decodeData(t, getJSON(t, env.url("/api/forge/account/list")), &accounts)
	acctList := accounts.List
	if len(acctList) != 1 || acctList[0].Token != forge.TokenMasked {
		t.Fatalf("list 应打码 token: %+v", acctList)
	}

	// 掩码值提交 = 未修改（token 保持原文 secret，list 仍打码）
	postJSON(t, env.url("/api/forge/account/save"), `{"forgeHost":"github.com","username":"heyuuu","token":"`+string(forge.TokenMasked)+`"}`)
	decodeData(t, getJSON(t, env.url("/api/forge/account/list")), &accounts)
	if len(accounts.List) != 1 || accounts.List[0].Token != forge.TokenMasked {
		t.Fatalf("掩码沿用后仍应打码: %+v", accounts.List)
	}

	// 坏数据：forge 未配置 → ok=false + 中文错误
	r = postJSON(t, env.url("/api/forge/account/save"), `{"forgeHost":"ghost.com","username":"x","token":"t"}`)
	if r.Ok || r.Message == "" {
		t.Fatalf("坏数据应失败并带中文错误: %+v", r)
	}

	r = postJSON(t, env.url("/api/forge/account/delete"), `{"forgeHost":"github.com","username":"heyuuu"}`)
	if !r.Ok {
		t.Fatalf("删除应成功: %s", r.Message)
	}
}

// TestForgeNamespaceApi namespace 出口契约：保存/类型校验/删除/未拉取对账报错。
func TestForgeNamespaceApi(t *testing.T) {
	env := newTestEnv(t)
	postJSON(t, env.url("/api/forge/save"), `{"host":"github.com","kind":"github"}`)

	r := postJSON(t, env.url("/api/forge/namespace/save"), `{"forgeHost":"github.com","path":"/heyuuu/","type":"personal"}`)
	if !r.Ok {
		t.Fatalf("保存应成功: %s", r.Message)
	}
	var namespaces web.ListResult[forge.Namespace]
	decodeData(t, getJSON(t, env.url("/api/forge/namespace/list")), &namespaces)
	nsList := namespaces.List
	if len(nsList) != 1 || nsList[0].Path != "heyuuu" {
		t.Fatalf("namespace 应归一化存储: %+v", namespaces)
	}

	// 坏数据：未知 type
	r = postJSON(t, env.url("/api/forge/namespace/save"), `{"forgeHost":"github.com","path":"a","type":"team"}`)
	if r.Ok || r.Message == "" {
		t.Fatalf("未知 type 应报中文错误: %+v", r)
	}

	// 未拉取就对账 → 中文错误（不触发外呼）
	resp, err := http.Get(env.url("/api/forge/namespace/reconcile?forgeHost=github.com&path=heyuuu"))
	if err != nil {
		t.Fatalf("reconcile 请求失败: %v", err)
	}
	defer resp.Body.Close()
	var env2 envelope
	if err := json.NewDecoder(resp.Body).Decode(&env2); err != nil {
		t.Fatalf("reconcile 响应解码失败: %v", err)
	}
	if env2.Ok || env2.Message == "" {
		t.Fatalf("未拉取应对账失败并提示: %+v", env2)
	}

	r = postJSON(t, env.url("/api/forge/namespace/delete"), `{"forgeHost":"github.com","path":"heyuuu"}`)
	if !r.Ok {
		t.Fatalf("删除应成功: %s", r.Message)
	}
}
