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
	register("project/list", h.List)
	register("project/info", h.Info)
}

func (h *ProjectHandler) List(params any) (result any, err error) {
	projects := h.service.Projects()
	list := slicekit.Map(projects, converter.ToProjectResponseDto)
	return listResult(list), nil
}

func (h *ProjectHandler) Info(params any) (result any, err error) {
	type infoParams struct {
		Name string `json:"name"`
	}

	// 将 params 转换为结构体
	p, err := parseParam[infoParams](params)
	if err != nil {
		return nil, err
	}

	app := h.service.FindByName(p.Name)
	return itemResult(app, converter.ToProjectResponseDto)
}
