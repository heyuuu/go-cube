package handlers

// handlers 层集成测试基建：拉起真实 web.Server（huma 路由 + envelope + nil 序列化），
// 打真实 HTTP 请求验证整条链路。服务层用 testfixture 构造（真实 git 仓库 + 临时 cache 目录），
// opener 执行注入 fake，断言组装的命令而不真正启动编辑器。
//
// 这里只测「HTTP 出口」的契约（路由 / DTO / envelope / 状态码）；业务规则的深测在各自
// domain 包的单测里，此处只构造能让 handler 走到目标分支的最小场景。
// 服务端框架本身的测试（静态资源 / SPA fallback / system 端点）在 cube/web 包。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"cube/config"
	"cube/internal/testfixture"
	"cube/opener"
	"cube/project"
	"cube/settings"
	"cube/usage"
	"cube/web"
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

	settingsFile := ws.Join("settings.json")
	if err := settings.SaveSection(settingsFile, "openers", []opener.Spec{
		{Name: "finder", Actions: map[opener.Role]string{"open-dir": "exec: /usr/bin/open $0"}},
		{Name: "broken"}, // 缺 commands，解析失败被跳过
	}); err != nil {
		t.Fatalf("写入测试 settings.json 失败: %v", err)
	}
	if err := settings.SaveSection(settingsFile, "scanRules", []project.ScanRule{
		{Group: "g1", Path: ws.Join("g1"), MaxDepth: 1},
		{Group: "g2", Path: ws.Join("g2"), MaxDepth: 1},
	}); err != nil {
		t.Fatalf("写入测试 settings.json 失败: %v", err)
	}
	if err := settings.SaveSection(settingsFile, "cloneRules", []project.CloneRule{
		{RepoHost: "github.com", LocalPath: ws.Join("repo")},
	}); err != nil {
		t.Fatalf("写入测试 settings.json 失败: %v", err)
	}

	projSvc := project.NewService(settingsFile, ws.Join("cache"))
	exec := &fakeExecutor{}
	openerSvc := opener.NewService(settingsFile, exec, "http://127.0.0.1:6001")
	usageSvc := usage.NewService(ws.Join("usage.jsonl"))
	cfg := &config.Config{
		DataDir: ws.Join("data"),
	}

	srv := web.NewServer(
		config.ServerConfig{Port: 6101},
		[]web.Handler{
			NewProjectHandler(projSvc, openerSvc, usageSvc),
			NewOpenerHandler(openerSvc),
			NewConfigHandler(cfg),
			NewMdHandler(),
			NewWorkbenchHandler(workbench.NewService(nil)),
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
