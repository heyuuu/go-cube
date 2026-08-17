package web

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
)

// uiFS 前端构建产物（make build-ui 把 web/dist 的内容拷到这里，go:embed 嵌入）
//
//go:embed ui/*
var uiFS embed.FS

// registerStaticRoutes 挂载前端静态资源与 SPA fallback：
//   - GET /assets/*   → Vite 构建产物（文件名带内容 hash，设 immutable 长缓存）
//   - GET /<文件>     → dist 根级文件（favicon.svg 等），存在即返回 
//   - GET 其它路径    → index.html（history 路由 fallback，支持 /projects 直达/刷新）
//   - /api/*、/docs、/openapi.json 的未命中**不走 fallback**，按 404 处理——
//     否则 API 打错路径会拿到 HTML 200，错误被吞成莫名的解析失败
func registerStaticRoutes(mux *http.ServeMux) {
	indexFile, err := fs.ReadFile(uiFS, "ui/index.html")
	if err != nil {
		slog.Error("ui assets 为空，无法挂载静态资源", "err", err)
		return
	}

	rootFS, err := fs.Sub(uiFS, "ui")
	if err != nil {
		slog.Error("无法进入 ui 子目录", "err", err)
		return
	}

	// /assets/* —— Vite 产物文件名带内容 hash，可不可变长缓存
	// （FileServer 不剥挂载前缀，须 StripPrefix 把 /assets/ 映射到 assetsFS 根）
	if assetsFS, err := fs.Sub(rootFS, "assets"); err == nil {
		mux.Handle("/assets/", cacheImmutable(http.StripPrefix("/assets/", http.FileServer(http.FS(assetsFS)))))
	} else {
		slog.Error("ui/assets 目录缺失，静态资源未挂载", "err", err)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if isNonFallbackPath(r.URL.Path) || !(r.Method == http.MethodGet || r.Method == http.MethodHead) {
			http.NotFound(w, r)
			return
		}
		// dist 根级文件（favicon.svg 等）存在则返回原文件，否则按 SPA 路由回 index.html
		if f, err := rootFS.Open(strings.TrimPrefix(r.URL.Path, "/")); err == nil {
			f.Close()
			http.FileServer(http.FS(rootFS)).ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexFile)
	})
}

// isNonFallbackPath 判定不参与 SPA fallback 的路径：API 与文档端点（未命中应 404 而非回退页面）
func isNonFallbackPath(p string) bool {
	return strings.HasPrefix(p, "/api/") ||
		p == "/docs" || strings.HasPrefix(p, "/docs/") ||
		p == "/openapi.json"
}

// cacheImmutable 包装 immutable 长缓存（仅用于文件名带内容 hash 的资源响应）
func cacheImmutable(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(w, r)
	})
}
