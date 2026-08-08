package web

import (
	"errors"
	"fmt"

	"github.com/danielgtaylor/huma/v2"

	"cube/config"
)

type ConfigHandler struct {
	conf *config.Config
}

func NewConfigHandler(conf config.Config) *ConfigHandler {
	return &ConfigHandler{conf: &conf}
}

// NewConfigHandlerPtr 接收 *Config 指针，供需要写回配置的场景使用。
func NewConfigHandlerPtr(conf *config.Config) *ConfigHandler {
	return &ConfigHandler{conf: conf}
}

func (h *ConfigHandler) Register(api huma.API) {
	apiGet(api, "/api/config", "获取配置信息", h.getConfig)

	// scan 规则增删（同 path 不同 method 需显式 operationId 避免 huma 冲突）
	apiRegisterOp(api, "POST", "/api/config/scan", "添加 scan 规则", "config.scan.add", h.addScanRule)
	apiRegisterOp(api, "DELETE", "/api/config/scan", "删除 scan 规则", "config.scan.del", h.delScanRule)

	// clone 规则增删
	apiRegisterOp(api, "POST", "/api/config/clone", "添加 clone 规则", "config.clone.add", h.addCloneRule)
	apiRegisterOp(api, "DELETE", "/api/config/clone", "删除 clone 规则", "config.clone.del", h.delCloneRule)

	// opener 增删
	apiRegisterOp(api, "POST", "/api/config/opener", "添加 opener", "config.opener.add", h.addOpener)
	apiRegisterOp(api, "DELETE", "/api/config/opener", "删除 opener", "config.opener.del", h.delOpener)
}

func (h *ConfigHandler) getConfig(_ struct{}) (config.Config, error) {
	return *h.conf, nil
}

// --- scan 规则 ---

type ScanRuleInput struct {
	Body config.ScanRuleConfig
}

// ScanRuleDeleteInput 删除 scan 规则的入参：只需 group + path 定位（maxDepth 可空）。
type ScanRuleDeleteInput struct {
	Body struct {
		Group string `json:"group"`
		Path  string `json:"path"`
	}
}

func (h *ConfigHandler) addScanRule(input ScanRuleInput) (config.ProjectConfig, error) {
	rule := input.Body
	if rule.Group == "" || rule.Path == "" {
		return config.ProjectConfig{}, errors.New("group 和 path 不能为空")
	}
	h.conf.Project.Scan = append(h.conf.Project.Scan, rule)
	if err := h.save(); err != nil {
		return config.ProjectConfig{}, err
	}
	return h.conf.Project, nil
}

func (h *ConfigHandler) delScanRule(input ScanRuleDeleteInput) (config.ProjectConfig, error) {
	g, p := input.Body.Group, input.Body.Path
	h.conf.Project.Scan = filterScan(h.conf.Project.Scan, func(r config.ScanRuleConfig) bool {
		return !(r.Group == g && r.Path == p)
	})
	if err := h.save(); err != nil {
		return config.ProjectConfig{}, err
	}
	return h.conf.Project, nil
}

// --- clone 规则 ---

type CloneRuleInput struct {
	Body config.CloneRuleConfig
}

func (h *ConfigHandler) addCloneRule(input CloneRuleInput) (config.ProjectConfig, error) {
	rule := input.Body
	if rule.RepoHost == "" || rule.LocalPath == "" {
		return config.ProjectConfig{}, errors.New("repoHost 和 localPath 不能为空")
	}
	h.conf.Project.Clone = append(h.conf.Project.Clone, rule)
	if err := h.save(); err != nil {
		return config.ProjectConfig{}, err
	}
	return h.conf.Project, nil
}

func (h *ConfigHandler) delCloneRule(input CloneRuleInput) (config.ProjectConfig, error) {
	rule := input.Body
	h.conf.Project.Clone = filterClone(h.conf.Project.Clone, func(r config.CloneRuleConfig) bool {
		return !(r.RepoHost == rule.RepoHost && r.RepoPrefix == rule.RepoPrefix && r.LocalPath == rule.LocalPath)
	})
	if err := h.save(); err != nil {
		return config.ProjectConfig{}, err
	}
	return h.conf.Project, nil
}

// --- opener ---

type OpenerInput struct {
	Body config.OpenerConfig
}

// OpenerDeleteInput 删除 opener 的入参：只需 name 定位。
type OpenerDeleteInput struct {
	Body struct {
		Name string `json:"name"`
	}
}

func (h *ConfigHandler) addOpener(input OpenerInput) ([]config.OpenerConfig, error) {
	op := input.Body
	if op.Name == "" || len(op.Cmd) == 0 {
		return nil, errors.New("name 和 cmd 不能为空")
	}
	// 同名覆盖（先删后加）
	h.conf.Openers = filterOpener(h.conf.Openers, func(o config.OpenerConfig) bool {
		return o.Name != op.Name
	})
	h.conf.Openers = append(h.conf.Openers, op)
	if err := h.save(); err != nil {
		return nil, err
	}
	return h.conf.Openers, nil
}

func (h *ConfigHandler) delOpener(input OpenerDeleteInput) ([]config.OpenerConfig, error) {
	name := input.Body.Name
	h.conf.Openers = filterOpener(h.conf.Openers, func(o config.OpenerConfig) bool {
		return o.Name != name
	})
	if err := h.save(); err != nil {
		return nil, err
	}
	return h.conf.Openers, nil
}

// --- helpers ---

func (h *ConfigHandler) save() error {
	if err := config.Save(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}
	return nil
}

func filterScan(rules []config.ScanRuleConfig, keep func(config.ScanRuleConfig) bool) []config.ScanRuleConfig {
	result := make([]config.ScanRuleConfig, 0, len(rules))
	for _, r := range rules {
		if keep(r) {
			result = append(result, r)
		}
	}
	return result
}

func filterClone(rules []config.CloneRuleConfig, keep func(config.CloneRuleConfig) bool) []config.CloneRuleConfig {
	result := make([]config.CloneRuleConfig, 0, len(rules))
	for _, r := range rules {
		if keep(r) {
			result = append(result, r)
		}
	}
	return result
}

func filterOpener(ops []config.OpenerConfig, keep func(config.OpenerConfig) bool) []config.OpenerConfig {
	result := make([]config.OpenerConfig, 0, len(ops))
	for _, o := range ops {
		if keep(o) {
			result = append(result, o)
		}
	}
	return result
}
