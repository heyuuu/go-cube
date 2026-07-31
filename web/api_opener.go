package web

import (
	"github.com/danielgtaylor/huma/v2"

	"github.com/heyuuu/cube/opener"
	"github.com/heyuuu/cube/util/slicekit"
)

// --- dto ---

type OpenerDTO struct {
	Name  string   `json:"name"`
	Cmd   []string `json:"cmd"`
	Roles []string `json:"roles"`
}

func toOpenerDTO(entity *opener.Opener) *OpenerDTO {
	if entity == nil {
		return nil
	}

	return &OpenerDTO{
		Name: entity.Name(),
		Cmd:  entity.Cmd(),
		Roles: slicekit.Map(entity.Roles(), func(r opener.Role) string {
			return string(r)
		}),
	}
}

// --- handler ---

type OpenerHandler struct {
	service *opener.Service
}

func NewOpenerHandler(service *opener.Service) *OpenerHandler {
	return &OpenerHandler{
		service: service,
	}
}

func (h *OpenerHandler) Register(api huma.API) {
	apiGet(api, "/api/opener/list", "获取 opener 列表", h.openerList)
	apiGet(api, "/api/opener/info", "获取 opener 详情", h.openerInfo)
}

func (h *OpenerHandler) openerList(_ struct{}) (ListResult[*OpenerDTO], error) {
	apps := h.service.AllOpeners()
	list := slicekit.Map(apps, toOpenerDTO)
	return listResult(list), nil
}

func (h *OpenerHandler) openerInfo(input struct {
	Name string `json:"name"`
}) (*OpenerDTO, error) {
	o := h.service.FindByName(input.Name)
	return toOpenerDTO(o), nil
}
