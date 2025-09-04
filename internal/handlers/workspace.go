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
	register("workspaces", h.Workspaces)
}

func (h *WorkspaceHandler) Workspaces(params any) (result any, err error) {
	workspaces := h.service.Workspaces()
	list := slicekit.Map(workspaces, converter.ToWorkspaceResponseDto)
	return listResult(list), nil
}
