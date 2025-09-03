package handlers

import (
	"github.com/heyuuu/go-cube/internal/services"
)

type ConfigHandler struct {
	service *services.ConfigService
}

func NewConfigHandler(service *services.ConfigService) *ConfigHandler {
	return &ConfigHandler{
		service: service,
	}
}

func (h *ConfigHandler) Register(register func(name string, handler HandleFunc)) {
	register("config", h.Get)
}

func (h *ConfigHandler) Get(params any) (result any, err error) {
	return h.service.Config(), nil
}
