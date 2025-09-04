package handlers

import (
	"github.com/heyuuu/go-cube/internal/converter"
	"github.com/heyuuu/go-cube/internal/services"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
)

type ApplicationHandler struct {
	service *services.ApplicationService
}

func NewApplicationHandler(service *services.ApplicationService) *ApplicationHandler {
	return &ApplicationHandler{
		service: service,
	}
}

func (h *ApplicationHandler) Register(register func(name string, handler HandleFunc)) {
	register("application/list", h.List)
	register("application/info", h.Info)
}

func (h *ApplicationHandler) List(params any) (result any, err error) {
	apps := h.service.Apps()
	list := slicekit.Map(apps, converter.ToApplicationResponseDto)
	return listResult(list), nil
}

func (h *ApplicationHandler) Info(params any) (result any, err error) {
	type infoParams struct {
		Name string `json:"name"`
	}

	// 将 params 转换为结构体
	p, err := parseParam[infoParams](params)
	if err != nil {
		return nil, err
	}

	app := h.service.FindByName(p.Name)
	return itemResult(app, converter.ToApplicationResponseDto)
}
