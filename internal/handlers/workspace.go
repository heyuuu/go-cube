package handlers

import (
	"github.com/heyuuu/go-cube/internal/converter"
	"github.com/heyuuu/go-cube/internal/services"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
)

type WorkspaceHandler struct {
	service *services.WorkspaceService
}

func NewWorkspaceHandler(service *services.WorkspaceService) *WorkspaceHandler {
	return &WorkspaceHandler{
		service: service,
	}
}

func (h *WorkspaceHandler) Register(register func(name string, handler HandleFunc)) {
	register("workspace/list", h.List)
	register("workspace/info", h.Info)
}

func (h *WorkspaceHandler) List(params any) (result any, err error) {
	workspaces := h.service.Workspaces()
	list := slicekit.Map(workspaces, converter.ToWorkspaceResponseDto)
	return listResult(list), nil
}

func (h *WorkspaceHandler) Info(params any) (result any, err error) {
	type infoParams struct {
		Name string `json:"name"`
	}

	// 将 params 转换为结构体
	p, err := parseParam[infoParams](params)
	if err != nil {
		return nil, err
	}

	app := h.service.FindByName(p.Name)
	return itemResult(app, converter.ToWorkspaceResponseDto)
}
