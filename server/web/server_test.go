package web

// web 层集成测试基建：拉起真实 Server（huma 路由 + envelope + nil 序列化 + 静态资源），
// 打真实 HTTP 请求验证整条链路。服务层用 testfixture 构造（真实 git 仓库 + 临时 cache 目录），
// opener 执行注入 fake，断言组装的命令而不真正启动编辑器。
//
// 这里只测「HTTP 出口」的契约（路由 / DTO / envelope / 状态码）；业务规则的深测在各自
// domain 包的单测里，此处只构造能让 handler 走到目标分支的最小场景。

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"cube/config"
	"cube/internal/testfixture"
	"cube/opener"
	"cube/project"
	"cube/settings"
	"cube/workbench"
)

// --- 测试环境 ---

type testEnv struct {
	ts        *httptest.Server
	ws        *testfixture.Workspace
	exec      *fakeExecutor
	projSvc   *project.Service
	openerSvc *opener.Service
	cfg       *config.Config
}

// newTestEnv 建两个项目的扫描环境（g1/proj1 普通、g2/proj2 带 godot tag）+ 一个 finder opener。
// 另配一个缺 cmd 的 broken opener，覆盖配置降级（跳过不阻断）。
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ws := testfixture.NewWorkspace(t)
	ws.MakeProjectDir("g1/proj1")
	ws.MakeProjectDir("g2/proj2", testfixture.WithGodot())

	scanCfg := []config.ScanRuleConfig{
		{Group: "g1", Path: ws.Join("g1"), MaxDepth: 1},
		{Group: "g2", Path: ws.Join("g2"), MaxDepth: 1},
	}
	cloneCfg := []config.CloneRuleConfig{
		{RepoHost: "github.com", LocalPath: ws.Join("repo")},
	}
	settingsFile := ws.Join("settings.json")
	if err := settings.SaveSection(settingsFile, "openers", []opener.Spec{
		{Name: "finder", Cmd: []string{"/usr/bin/open", "$0"}, Roles: []string{"open-dir"}},
		{Name: "broken", Cmd: []string{}}, // 缺 cmd，解析失败被跳过
	}); err != nil {
		t.Fatalf("写入测试 settings.json 失败: %v", err)
	}

	projSvc := project.NewService(config.ProjectConfig{Scan: scanCfg, Clone: cloneCfg}, ws.Join("cache"))
	exec := &fakeExecutor{}
	openerSvc := opener.NewService(settingsFile, "http://localhost:6101", exec)
	cfg := &config.Config{
		DataDir: ws.Join("data"),
		Project: config.ProjectConfig{Scan: scanCfg, Clone: cloneCfg},
	}

	srv := NewServer(
		config.ServerConfig{Port: 6101},
		[]Handler{
			NewProjectHandler(projSvc),
			NewOpenerHandler(openerSvc),
			NewConfigHandler(cfg),
			NewMdHandler(),
			NewWorkbenchHandler(workbench.NewService()),
		},
	)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	return &testEnv{ts: ts, ws: ws, exec: exec, projSvc: projSvc, openerSvc: openerSvc, cfg: cfg}
}

// fakeExecutor 记录组装完成的命令（不真正执行）。
type fakeExecutor struct {
	mu    sync.Mutex
	calls [][]string
}

func (f *fakeExecutor) Run(bin string, args ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string{bin}, args...))
	return nil
}

func (f *fakeExecutor) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeExecutor) lastCall() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return nil
	}
	return f.calls[len(f.calls)-1]
}

// --- HTTP 断言辅助 ---

// envelope 与 ApiOutput 的 JSON 形态对应；data 延迟到调用方按需解码。
type envelope struct {
	Ok      bool            `json:"ok"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// getJSON 打 GET 请求并断言 200 + envelope 解码成功，返回 envelope。
func getJSON(t *testing.T, url string) envelope {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s 失败: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s 应为 200, got %d", url, resp.StatusCode)
	}
	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("GET %s 响应不是合法 envelope: %v", url, err)
	}
	return env
}

// decodeData 把 envelope.data 解码到目标类型。
func decodeData[T any](t *testing.T, env envelope, out *T) {
	t.Helper()
	if !env.Ok {
		t.Fatalf("envelope.ok 应为 true, message=%q", env.Message)
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		t.Fatalf("envelope.data 解码失败: %v, raw=%s", err, env.Data)
	}
}

func (e *testEnv) url(path string) string { return e.ts.URL + path }

func (e *testEnv) proj1Path() string { return e.ws.Join("g1", "proj1") }

// --- system ---

func TestWhoami(t *testing.T) {
	env := newTestEnv(t)
	env2 := getJSON(t, env.url("/api/system/whoami"))
	var got struct {
		App     string `json:"app"`
		Version string `json:"version"`
	}
	decodeData(t, env2, &got)
	if got.App != "cube" {
		t.Errorf("app 应为 cube, got %q", got.App)
	}
	if got.Version == "" {
		t.Error("version 不应为空")
	}
}

// TestShutdownUnauthorized shutdown 鉴权失败回 401。
// 鉴权通过路径会给本进程发 SIGTERM，无法在测试里安全覆盖（shutdown_token_test 已单测 token 逻辑）。
func TestShutdownUnauthorized(t *testing.T) {
	env := newTestEnv(t)
	resp, err := http.Post(env.url("/api/system/shutdown"), "", nil)
	if err != nil {
		t.Fatalf("POST shutdown 失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("无 token 的 shutdown 应为 401, got %d", resp.StatusCode)
	}
}

// --- openapi 与静态资源 ---

func TestOpenAPIJSON(t *testing.T) {
	env := newTestEnv(t)
	resp, err := http.Get(env.url("/openapi.json"))
	if err != nil {
		t.Fatalf("GET /openapi.json 失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/openapi.json 应为 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	spec := string(body)
	if !contains(spec, "/api/project/list") {
		t.Error("spec 应包含 /api/project/list")
	}
	// 回归：tree 接口已移除，spec 不应再出现
	if contains(spec, "/api/project/tree") {
		t.Error("spec 不应包含已移除的 /api/project/tree")
	}
}

func TestStaticSPAFallback(t *testing.T) {
	env := newTestEnv(t)
	for _, path := range []string{"/", "/projects", "/projects?view=tree", "/config"} {
		resp, err := http.Get(env.url(path))
		if err != nil {
			t.Fatalf("GET %s 失败: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s 应为 200, got %d", path, resp.StatusCode)
			continue
		}
		if !contains(string(body), "<!doctype html") {
			t.Errorf("GET %s 应回退 index.html, body=%q", path, truncate(body, 80))
		}
	}
}

// TestStaticAPIPathNoFallback API 路径未命中必须 404 而非回退 HTML，
// 否则打错路径的前端拿到 HTML 200，错误被吞成解析失败。
func TestStaticAPIPathNoFallback(t *testing.T) {
	env := newTestEnv(t)
	for _, path := range []string{"/api/not-exist", "/docs/", "/openapi.json "} {
		resp, err := http.Get(env.url(path))
		if err != nil {
			t.Fatalf("GET %s 失败: %v", path, err)
		}
		resp.Body.Close()
		// /openapi.json 带尾空格会被 ServeMux 清洗后命中 openapi.json 本身，跳过该断言
		if path == "/openapi.json " && resp.StatusCode == http.StatusOK {
			continue
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s 应为 404（不参与 fallback）, got %d", path, resp.StatusCode)
		}
	}
}

func TestStaticRootFileAndAssets(t *testing.T) {
	env := newTestEnv(t)

	// dist 根级文件存在即返回原文件
	resp, err := http.Get(env.url("/favicon.svg"))
	if err != nil {
		t.Fatalf("GET /favicon.svg 失败: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/favicon.svg 应为 200, got %d", resp.StatusCode)
	}

	// /assets/* 带 immutable 长缓存（文件名含内容 hash）
	asset := firstAssetName(t)
	if asset == "" {
		t.Skip("ui/assets 为空（未构建前端），跳过 assets 缓存断言")
	}
	resp2, err := http.Get(env.url("/assets/" + asset))
	if err != nil {
		t.Fatalf("GET /assets/%s 失败: %v", asset, err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("/assets/%s 应为 200, got %d", asset, resp2.StatusCode)
	}
	if cc := resp2.Header.Get("Cache-Control"); !contains(cc, "immutable") {
		t.Errorf("/assets/* 应带 immutable 缓存头, got %q", cc)
	}
}

// firstAssetName 从嵌入产物里取一个 assets 文件名。
func firstAssetName(t *testing.T) string {
	t.Helper()
	entries, err := fs.ReadDir(uiFS, "ui/assets")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			return e.Name()
		}
	}
	return ""
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && strings.Contains(s, sub)
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "..."
	}
	return string(b)
}

func TestServerPortAndURL(t *testing.T) {
	srv := NewServer(config.ServerConfig{Port: 6101}, nil)
	if srv.Port() != 6101 {
		t.Errorf("Port() = %d, want 6101", srv.Port())
	}
	if got := srv.ServerURL(); got != "http://localhost:6101/" {
		t.Errorf("ServerURL() = %q, want http://localhost:6101/", got)
	}
}

func TestServerStartWithoutPort(t *testing.T) {
	srv := NewServer(config.ServerConfig{}, nil)
	if err := srv.Start(); err == nil {
		t.Fatal("port=0 时 Start 应返回中文错误，得到 nil")
	}
}
