package web

import (
	"github.com/danielgtaylor/huma/v2"

	"cube/config"
)

type ConfigHandler struct {
	conf *config.Config
}

// NewConfigHandler 接收 *Config 指针，供需要写回配置的场景使用。
func NewConfigHandler(conf *config.Config) *ConfigHandler {
	return &ConfigHandler{conf: conf}
}

func (h *ConfigHandler) Register(api huma.API) {
	apiGet(api, "/api/config", "获取配置信息", h.getConfig)
}

func (h *ConfigHandler) getConfig(_ struct{}) (config.Config, error) {
	return *h.conf, nil
}
