package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cube/web"
)

// startMockCubeServer 起一个 httptest server 模拟 cube 的 whoami 端点。
// 返回 (server, port)。status==200 且 app=="cube" 时被 Status 判为在跑。
func startMockCubeServer(t *testing.T, version string) (*httptest.Server, int) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/whoami", func(w http.ResponseWriter, r *http.Request) {
		body := struct {
			Ok   bool `json:"ok"`
			Data struct {
				App      string `json:"app"`
				Version  string `json:"version"`
				Instance string `json:"instance"`
			} `json:"data"`
		}{
			Ok: true,
			Data: struct {
				App      string `json:"app"`
				Version  string `json:"version"`
				Instance string `json:"instance"`
			}{App: "cube", Version: version, Instance: "mock-inst"},
		}
		_ = json.NewEncoder(w).Encode(body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	// 从 srv.URL（http://127.0.0.1:PORT）解析端口
	port := 0
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)
	return srv, port
}

func TestStatus_Running(t *testing.T) {
	_, port := startMockCubeServer(t, "v9.9.9")
	st := Status(port)
	if !st.Running {
		t.Error("cube server 在跑应判 Running=true")
	}
	if st.Version != "v9.9.9" {
		t.Errorf("version 应为 v9.9.9，got %s", st.Version)
	}
	if st.Instance != "mock-inst" {
		t.Errorf("instance 应为 mock-inst，got %s", st.Instance)
	}
}

func TestStatus_NotRunning(t *testing.T) {
	// 端口 1 几乎肯定没监听
	if st := Status(1); st.Running {
		t.Error("连不上应判 Running=false")
	}
}

func TestStatus_NotCube(t *testing.T) {
	// 模拟别的服务（app != "cube"）
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/whoami", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"not-cube","version":"x"}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)

	if st := Status(port); st.Running {
		t.Error("app!=cube 应判 Running=false（端口上的不是 cube）")
	}
}

func TestStop_NotRunning(t *testing.T) {
	stopped, _, err := Stop(1) // 端口 1 没监听
	if err != nil {
		t.Errorf("停一个没在跑的 server 不应报错: %v", err)
	}
	if stopped {
		t.Error("没在跑应返回 stopped=false")
	}
}

// TestStop_ShutdownAndConfirm 验证 Stop 能触发 shutdown 端点并确认旧实例下线。
// 用一个可控的 mock server：收到 shutdown 后停止响应 whoami（端口空出，无新实例接管）。
func TestStop_ShutdownAndConfirm(t *testing.T) {
	alive := true
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/whoami", func(w http.ResponseWriter, r *http.Request) {
		if !alive {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"cube","version":"v1","instance":"old-inst"}}`))
	})
	mux.HandleFunc("/api/system/shutdown", func(w http.ResponseWriter, r *http.Request) {
		// 校验鉴权 header（复用 web.VerifyShutdownToken）
		if err := web.VerifyShutdownToken(r.Header.Get(web.ShutdownTokenHeader), time.Now()); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		alive = false // 模拟 shutdown 生效
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)

	stopped, replaced, err := Stop(port)
	if err != nil {
		t.Fatalf("Stop 应成功: %v", err)
	}
	if !stopped {
		t.Error("应返回 stopped=true")
	}
	if replaced {
		t.Error("端口空出无接管，应返回 replaced=false")
	}
	if alive {
		t.Error("mock server 应已被 shutdown（alive 应为 false）")
	}
}

// TestStop_ReplacedByNewInstance 模拟 launchctl 保活：旧实例 shutdown 后新实例立刻占回端口。
// Stop 应按 instance 换人识别旧实例已下线，返回 stopped=true, replaced=true。
func TestStop_ReplacedByNewInstance(t *testing.T) {
	instance := "old-inst"
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/whoami", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fmt.Sprintf(`{"ok":true,"data":{"app":"cube","version":"v1","instance":%q}}`, instance)))
	})
	mux.HandleFunc("/api/system/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if err := web.VerifyShutdownToken(r.Header.Get(web.ShutdownTokenHeader), time.Now()); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		instance = "new-inst" // 模拟新进程接管端口
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)

	stopped, replaced, err := Stop(port)
	if err != nil {
		t.Fatalf("Stop 应成功: %v", err)
	}
	if !stopped {
		t.Error("旧实例已 shutdown，应返回 stopped=true")
	}
	if !replaced {
		t.Error("端口被新实例接管，应返回 replaced=true")
	}
}
