package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"cube/version"
)

// SystemHandler 提供 /api/system/* 端点：服务自身的内部管理 API。
//
//	GET  /api/system/whoami   服务身份探活（无鉴权，只读）
//	POST /api/system/shutdown  触发服务平滑关闭（HMAC 时间戳鉴权，防 CSRF/重放）
//
// shutdown 不走 huma 注册（运维端点，不进 OpenAPI 文档），直接挂 ServeMux。
// SystemHandler 不持有 Server 引用——shutdown 通过给本进程发 SIGTERM 触发，
// 复用 Start 里已注册的信号监听路径。
type SystemHandler struct{}

func newSystemHandler() *SystemHandler {
	return &SystemHandler{}
}

func (h *SystemHandler) Register(api huma.API, mux *http.ServeMux) {
	// whoami：标准查询，进 huma 文档
	apiGet(api, "/api/system/whoami", "服务身份探活", h.whoami)

	// shutdown：直接挂 mux，不走 huma（鉴权定制 + 不进文档）
	mux.HandleFunc("POST /api/system/shutdown", h.handleShutdown)
}

// WhoamiResponse whoami 返回体。
type WhoamiResponse struct {
	App     string `json:"app"`     // 固定 "cube"，供探活方验证身份
	Version string `json:"version"` // cube 版本号
}

func (h *SystemHandler) whoami(_ struct{}) (WhoamiResponse, error) {
	return WhoamiResponse{App: "cube", Version: version.Version()}, nil
}

// handleShutdown 校验 X-Shutdown-Token 后给本进程发 SIGTERM 触发 graceful shutdown。
//
// 鉴权失败回 401，通过则回 200 再异步发信号（不在请求处理里阻塞，否则响应回不去）。
// 信号由 Start 的 signal.Notify 接收，走与用户 Ctrl+C 完全相同的关闭路径。
func (h *SystemHandler) handleShutdown(w http.ResponseWriter, r *http.Request) {
	tokenVal := r.Header.Get(ShutdownTokenHeader)
	if err := VerifyShutdownToken(tokenVal, time.Now()); err != nil {
		slog.Warn("shutdown 鉴权失败", "err", err, "remote", r.RemoteAddr)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "鉴权失败"})
		return
	}

	// 先响应客户端，再异步触发 shutdown——否则发 SIGTERM 后进程退出会截断响应
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"ok": "shutting down"})

	go func() {
		// 给本进程发 SIGTERM，由 Start 的信号监听统一处理 graceful shutdown
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()
}
