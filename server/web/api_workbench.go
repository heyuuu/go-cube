package web

import (
	"github.com/danielgtaylor/huma/v2"

	"cube/workbench"
)

// 工作台基座（提案 docs/proposals/1010-workbench基座）。
// 本文件先落 info / refs 两个接口；tree / file / diff / file-diff / commits
// 的契约在提案中定死，由 1011-1013 各自注册实现。

type WorkbenchHandler struct {
	workbenchService *workbench.Service
}

func NewWorkbenchHandler(workbenchService *workbench.Service) *WorkbenchHandler {
	return &WorkbenchHandler{workbenchService: workbenchService}
}

func (h *WorkbenchHandler) Register(api huma.API) {
	apiGet(api, "/api/workbench/info", "获取工作台项目信息", h.info)
	apiGet(api, "/api/workbench/refs", "获取工作台分支与tag列表", h.refs)
}

func (h *WorkbenchHandler) info(input struct {
	Path string `query:"path" required:"true"`
}) (*workbench.Info, error) {
	return h.workbenchService.Info(input.Path)
}

func (h *WorkbenchHandler) refs(input struct {
	Path string `query:"path" required:"true"`
}) (*workbench.Refs, error) {
	return h.workbenchService.Refs(input.Path)
}
