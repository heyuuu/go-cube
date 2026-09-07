package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"cube/config"
	"cube/version"
)

// Handler 接口
type Handler interface {
	Register(api huma.API, mux *http.ServeMux)
}

// Server 服务器，响应 api 请求
type Server struct {
	// config
	port int
	// runtime
	mux *http.ServeMux
	api huma.API
}

func NewServer(c config.ServerConfig, handlers []Handler) *Server {
	// 添加默认 Handler
	handlers = append(
		// 内置 handlers
		[]Handler{
			// system 端点（whoami / shutdown）
			newSystemHandler(),
			// 静态前端资源路由（/assets/* 与 SPA fallback）
			newStaticHandler(),
		},
		handlers...,
	)

	mux := http.NewServeMux()

	cfg := huma.DefaultConfig(version.AppTitle, version.Version())
	cfg.DocsRenderer = huma.DocsRendererScalar // 切换 /docs 页面风格为 Scalar 渲染器
	cfg.Formats = map[string]huma.Format{
		"application/json": nilSliceJSONFormat, // nil 切片/map → []/{}，避免前端拿到 null 崩溃
	}
	api := humago.New(mux, cfg)

	// 各 domain 注册自己的路由
	for _, handler := range handlers {
		handler.Register(api, mux)
	}

	return &Server{
		// config
		port: c.Port,
		// runtime
		mux: mux,
		api: api,
	}
}

func (s *Server) Port() int { return s.port }

// Handler 返回底层 http.Handler，供 httptest 拉起真实路由做集成测试。
func (s *Server) Handler() http.Handler { return s.mux }

// OpenAPIJSON 返回 OpenAPI 3.1 spec 的 JSON 字节。供 generate 命令或 /openapi.json 端点使用。
func (s *Server) OpenAPIJSON() ([]byte, error) {
	return s.api.OpenAPI().MarshalJSON()
}

// serverHost server 绑定的主机名——全仓 http 地址拼接的唯一事实源（web.BaseURL），
// 以后换域名/绑定时只改这里。
const serverHost = "localhost"

// BaseURL 按 port 拼 server 的基地址（无尾斜杠），全仓 http 地址拼接收敛于此。
func BaseURL(port int) string {
	return "http://" + serverHost + ":" + strconv.Itoa(port)
}

// ServerURL 访问地址
func (s *Server) ServerURL() string {
	return BaseURL(s.port) + "/"
}

// Start 启动 server，收到 SIGINT/SIGTERM 时优雅关闭。
func (s *Server) Start() error {
	if s.port == 0 {
		return fmt.Errorf("未配置服务端口号(配置项 config.Server.Port)")
	}

	addr := fmt.Sprintf(":%d", s.port)

	server := &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
		// 不设 Read/WriteTimeout：PTY WebSocket 与大文件上传都是长连接，
		// 整体超时会把合法会话掐断（慢客户端由 IdleTimeout 兜底）
		IdleTimeout: 120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server 启动", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// 接收信号关闭 server
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server 启动失败: %w", err)
	case sig := <-sigCh:
		slog.Info("server 关闭中", "signal", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
}
