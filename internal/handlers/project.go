package handlers

import (
	"github.com/heyuuu/go-cube/internal/converter"
	"github.com/heyuuu/go-cube/internal/services"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
)

type ProjectHandler struct {
	service *services.ProjectService
}

func NewProjectHandler(service *services.ProjectService) *ProjectHandler {
	return &ProjectHandler{
		service: service,
	}
}

func (h *ProjectHandler) Register(register func(name string, handler HandleFunc)) {
	register("projects", h.Projects)
}

func (h *ProjectHandler) Projects(params any) (result any, err error) {
	projects := h.service.Projects()
	list := slicekit.Map(projects, converter.ToProjectResponseDto)
	return listResult(list), nil
}
