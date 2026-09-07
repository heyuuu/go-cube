package handlers

import (
	"errors"
	"net/http"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"

	"cube/usage"
	"cube/web"
)

// --- handler ---

// UsageHandler usage 出口：读侧给 workbench 入口的「最近打开」，写侧补记
// web 直开（url 型 opener 动作、workbench 进入）这类不经过后端打开链路的使用信号。
type UsageHandler struct {
	service *usage.Service
}

func NewUsageHandler(service *usage.Service) *UsageHandler {
	return &UsageHandler{service: service}
}

func (h *UsageHandler) Register(api huma.API, mux *http.ServeMux) {
	web.ApiGet(api, "/api/usage/recent-paths", "获取最近使用的路径清单（去重取最新，最近使用倒序）", h.recentPaths)
	web.ApiPost(api, "/api/usage/record", "补记一条使用记录（web 直开不经过后端记录的场景，如 url 型 opener 动作、workbench 进入）", h.record)
}

func (h *UsageHandler) recentPaths(input struct {
	Limit int `query:"limit" doc:"返回条数上限，缺省 5"`
}) (web.ListResult[usage.PathUsage], error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 5
	}
	return listResult(h.service.RecentPaths(limit)), nil
}

// UsageRecordInput record 接口入参。字段对齐 usage.Record：
// workbench 进入只传 project（opener 留空——空 opener 不参与 opener 偏好统计）。
type UsageRecordInput struct {
	Body struct {
		Project string `json:"project,omitempty" doc:"项目/目录绝对路径（归并键）"`
		Opener  string `json:"opener,omitempty" doc:"opener 名（web 直开的 url 型动作补记时传）"`
		Dir     string `json:"dir,omitempty" doc:"实际打开的目标目录绝对路径，缺省记项目根"`
	}
}

func (h *UsageHandler) record(input UsageRecordInput) (map[string]any, error) {
	if input.Body.Project == "" || !filepath.IsAbs(input.Body.Project) {
		return nil, errors.New("project 必须是绝对路径: " + input.Body.Project)
	}
	if err := h.service.RecordOpen(input.Body.Project, input.Body.Opener, input.Body.Dir); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}
