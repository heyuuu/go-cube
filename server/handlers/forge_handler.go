package handlers

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"cube/forge"
	"cube/util/iconkit"
	"cube/web"
)

// ForgeHandler forge 配置管理的 HTTP 出口（提案 1040）。
type ForgeHandler struct {
	forgeService *forge.Service
}

func NewForgeHandler(forgeService *forge.Service) *ForgeHandler {
	return &ForgeHandler{forgeService: forgeService}
}

func (h *ForgeHandler) Register(api huma.API, mux *http.ServeMux) {
	web.ApiGet(api, "/api/forge/list", "获取 forge 列表", h.forgeList)
	web.ApiPost(api, "/api/forge/save", "新增或按 host 替换 forge", h.forgeSave)
	web.ApiPost(api, "/api/forge/delete", "按 host 删除 forge", h.forgeDelete)
	web.ApiPost(api, "/api/forge/reorder", "按 host 重排 forge 顺序", h.forgeReorder)
}

func (h *ForgeHandler) forgeList(_ struct{}) (web.ListResult[forge.Forge], error) {
	return listResult(h.forgeService.Forges()), nil
}

// ForgeSaveInput forge/save 接口入参（host 归一化后是唯一键）。
type ForgeSaveInput struct {
	Body struct {
		Host string   `json:"host" doc:"域名（可带端口），如 github.com"`
		Kind string   `json:"kind" doc:"API 方言：github / gitea / gitee / generic"`
		Icon *IconDTO `json:"icon,omitempty" doc:"平台图标（可选：lucide 图名或 base64 PNG）"`
	}
}

func (h *ForgeHandler) forgeSave(input ForgeSaveInput) (map[string]any, error) {
	f := forge.Forge{Host: input.Body.Host, Kind: input.Body.Kind}
	if input.Body.Icon != nil {
		f.Icon = &iconkit.Icon{Type: input.Body.Icon.Type, Value: input.Body.Icon.Value}
	}
	if err := h.forgeService.SaveForge(f); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// ForgeDeleteInput forge/delete 接口入参。
type ForgeDeleteInput struct {
	Body struct {
		Host string `json:"host" doc:"规则唯一键"`
	}
}

func (h *ForgeHandler) forgeDelete(input ForgeDeleteInput) (map[string]any, error) {
	if err := h.forgeService.DeleteForge(input.Body.Host); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// ForgeReorderInput forge/reorder 接口入参（整表按目标顺序提交 host 名单）。
type ForgeReorderInput struct {
	Body struct {
		Hosts []string `json:"hosts" doc:"按目标顺序排列的 host 名单"`
	}
}

func (h *ForgeHandler) forgeReorder(input ForgeReorderInput) (map[string]any, error) {
	if err := h.forgeService.ReorderForges(input.Body.Hosts); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}
