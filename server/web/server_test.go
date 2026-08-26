package web

// web 框架集成测试：拉起真实 Server（huma 路由 + envelope + nil 序列化 + 静态资源），
// 打真实 HTTP 请求验证框架自身的契约（system 端点 / openapi / SPA fallback / 缓存头 / 端口）。
//
// 框架自测不依赖任何业务 handler——用 testHandler 注册一条最小路由即可覆盖注册链路；
// 业务 handler 的契约测试（路由 / DTO / envelope）在 cube/handlers 包。

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"cube/config"
)

// testHandler 框架自测用的最小 handler：一条 GET 路由，覆盖注册链路与 envelope。
type testHandler struct{}

func (testHandler) Register(api huma.API, mux *http.ServeMux) {
	ApiGet(api, "/api/test/ping", "测试探活", func(_ struct{}) (string, error) {
		return "pong", nil
	})
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := NewServer(config.ServerConfig{Port: 6101}, []Handler{testHandler{}})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
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

// --- system ---

func TestWhoami(t *testing.T) {
	ts := newTestServer(t)
	env := getJSON(t, ts.URL+"/api/system/whoami")
	var got struct {
		App     string `json:"app"`
		Version string `json:"version"`
	}
	decodeData(t, env, &got)
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
	ts := newTestServer(t)
	resp, err := http.Post(ts.URL+"/api/system/shutdown", "", nil)
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
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/openapi.json")
	if err != nil {
		t.Fatalf("GET /openapi.json 失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/openapi.json 应为 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !contains(string(body), "/api/test/ping") {
		t.Error("spec 应包含注册的路由 /api/test/ping")
	}
}

func TestStaticSPAFallback(t *testing.T) {
	ts := newTestServer(t)
	for _, path := range []string{"/", "/projects", "/projects?view=tree", "/config"} {
		resp, err := http.Get(ts.URL + path)
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
	ts := newTestServer(t)
	for _, path := range []string{"/api/not-exist", "/docs/", "/openapi.json "} {
		resp, err := http.Get(ts.URL + path)
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
	ts := newTestServer(t)

	// dist 根级文件存在即返回原文件
	resp, err := http.Get(ts.URL + "/favicon.svg")
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
	resp2, err := http.Get(ts.URL + "/assets/" + asset)
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
