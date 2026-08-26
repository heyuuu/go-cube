package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestOpenerList(t *testing.T) {
	env := newTestEnv(t)
	var got struct {
		List []struct {
			Name    string   `json:"name"`
			Summary string   `json:"summary"`
			Roles   []string `json:"roles"`
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
	if op.Summary != "/usr/bin/open $0" {
		t.Errorf("summary 应保留 $0 占位符原样输出, got %q", op.Summary)
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
	env1 := post(`{"path":"` + env.ws.Join("notes") + `","opener":"finder"}`)
	if !env1.Ok {
		t.Fatalf("打开目录应成功, message=%q", env1.Message)
	}
	if env.exec.callCount() != 1 || env.exec.lastCall()[0] != "/usr/bin/open" {
		t.Errorf("executor 应收到 finder 命令, calls=%v", env.exec.calls)
	}

	// role 不匹配：finder 只有 open-dir，开文件应报错
	env2 := post(`{"path":"` + md + `","opener":"finder"}`)
	if env2.Ok || !strings.Contains(env2.Message, "open-file") {
		t.Errorf("open-dir opener 开文件应报 role 错误, got ok=%v message=%q", env2.Ok, env2.Message)
	}

	// 不存在的 opener
	env3 := post(`{"path":"` + md + `","opener":"nope"}`)
	if env3.Ok || !strings.Contains(env3.Message, "未找到指定 opener") {
		t.Errorf("未知 opener 应报错, got ok=%v message=%q", env3.Ok, env3.Message)
	}
}

func TestOpenerSaveAndDelete(t *testing.T) {
	env := newTestEnv(t)
	postSave := func(body string) envelope {
		resp, err := http.Post(env.url("/api/opener/save"), "application/json", bytes.NewReader([]byte(body)))
		if err != nil {
			t.Fatalf("POST opener/save 失败: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST opener/save 应为 200, got %d", resp.StatusCode)
		}
		var out envelope
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("响应解码失败: %v", err)
		}
		return out
	}

	// 新增（带 icon），list 应可见且 DTO 带回 icon
	env1 := postSave(`{"name":"code","cmd":["code","$0"],"roles":["open-dir"],"icon":{"type":"lucide","value":"app-window"}}`)
	if !env1.Ok {
		t.Fatalf("保存应成功, message=%q", env1.Message)
	}
	list := getJSON(t, env.url("/api/opener/list"))
	var got struct {
		List []struct {
			Name string `json:"name"`
			Icon *struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"icon"`
		} `json:"list"`
	}
	decodeData(t, list, &got)
	if len(got.List) != 2 { // fixture 的 finder + 新增 code
		t.Fatalf("应有 2 个 opener, got %d", len(got.List))
	}
	var code *struct {
		Name string `json:"name"`
		Icon *struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"icon"`
	}
	for i := range got.List {
		if got.List[i].Name == "code" {
			code = &got.List[i]
		}
	}
	if code == nil || code.Icon == nil || code.Icon.Value != "app-window" {
		t.Fatalf("新增 opener 的 type/icon 不符: %+v", code)
	}

	// 坏数据（缺 cmd）中文错误、不落文件
	env2 := postSave(`{"name":"bad","cmd":[]}`)
	if env2.Ok || !strings.Contains(env2.Message, "cmd") {
		t.Fatalf("坏数据应报 cmd 错误, got ok=%v message=%q", env2.Ok, env2.Message)
	}

	// 删除
	delResp, err := http.Post(env.url("/api/opener/delete"), "application/json", bytes.NewReader([]byte(`{"name":"code"}`)))
	if err != nil {
		t.Fatalf("POST opener/delete 失败: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("delete 应为 200, got %d", delResp.StatusCode)
	}
	var envDel envelope
	if err := json.NewDecoder(delResp.Body).Decode(&envDel); err != nil {
		t.Fatalf("删除响应解码失败: %v", err)
	}
	if !envDel.Ok {
		t.Fatalf("删除应成功, message=%q", envDel.Message)
	}
	list2 := getJSON(t, env.url("/api/opener/list"))
	decodeData(t, list2, &got)
	for _, o := range got.List {
		if o.Name == "code" {
			t.Fatal("code 应已被删除")
		}
	}
}

func TestOpenerExtractIcon(t *testing.T) {
	env := newTestEnv(t)
	// 造一个带 icns 的 .app（复用 iconkit 的容器格式）
	png1x1 := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0x0d, 'I', 'H', 'D', 'R'}
	// 1x1 PNG 无法缩放路径（≤64 原样返回），但解码要求完整 IHDR/IDAT/IEND——直接用最小合法 PNG
	png1x1 = []byte{
		0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
		0, 0, 0, 0x0d, 'I', 'H', 'D', 'R',
		0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1f, 0x15, 0xc4, 0x89,
		0, 0, 0, 0x0a, 'I', 'D', 'A', 'T', 0x78, 0x9c, 0x63, 0, 1, 0, 0, 5, 0, 1, 0x0d, 0x0a, 0x2d, 0xb4,
		0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82,
	}
	entry := make([]byte, 8+len(png1x1))
	copy(entry, "ic07")
	entry[4], entry[5], entry[6], entry[7] = byte(len(entry)>>24), byte(len(entry)>>16), byte(len(entry)>>8), byte(len(entry))
	copy(entry[8:], png1x1)
	headerLen := 8 + len(entry)
	icns := append([]byte("icns"), byte(headerLen>>24), byte(headerLen>>16), byte(headerLen>>8), byte(headerLen))
	icns = append(icns, entry...)
	env.ws.Mkdir("Fake.app", "Contents", "Resources")
	env.ws.WriteFile("Fake.app/Contents/Resources/Fake.icns", icns)

	body := `{"path":"` + env.ws.Join("Fake.app") + `"}`
	resp, err := http.Post(env.url("/api/opener/extract-icon"), "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST extract-icon 失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("extract-icon 应为 200, got %d", resp.StatusCode)
	}
	var out struct {
		Data struct {
			Value string `json:"value"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if dec, err := base64.StdEncoding.DecodeString(out.Data.Value); err != nil || len(dec) == 0 {
		t.Fatalf("value 应为合法 base64 PNG, err=%v len=%d", err, len(dec))
	}
}

func TestOpenerReorder(t *testing.T) {
	env := newTestEnv(t)
	postJSON := func(path, body string) envelope {
		resp, err := http.Post(env.url(path), "application/json", bytes.NewReader([]byte(body)))
		if err != nil {
			t.Fatalf("POST %s 失败: %v", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST %s 应为 200, got %d", path, resp.StatusCode)
		}
		var out envelope
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("响应解码失败: %v", err)
		}
		return out
	}

	// fixture 预置 finder；再存一条 code，reorder 后 list 顺序应随之变化
	postJSON("/api/opener/save", `{"name":"code","cmd":["code","$0"],"roles":["open-dir"]}`)
	postJSON("/api/opener/reorder", `{"names":["code","finder"]}`)

	list := getJSON(t, env.url("/api/opener/list"))
	var got struct {
		List []struct {
			Name string `json:"name"`
		} `json:"list"`
	}
	decodeData(t, list, &got)
	if len(got.List) != 2 || got.List[0].Name != "code" || got.List[1].Name != "finder" {
		t.Fatalf("reorder 后顺序不符: %+v", got.List)
	}

	// 未知名返回中文错误、顺序不变
	bad := postJSON("/api/opener/reorder", `{"names":["finder","nope"]}`)
	if bad.Ok || !strings.Contains(bad.Message, "nope") {
		t.Fatalf("未知名应报错, ok=%v message=%q", bad.Ok, bad.Message)
	}
	list2 := getJSON(t, env.url("/api/opener/list"))
	decodeData(t, list2, &got)
	if got.List[0].Name != "code" {
		t.Fatalf("失败 reorder 不应改动顺序: %+v", got.List)
	}
}
