package create

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// TemplateVersionCurrent 是当前引擎支持的 template.yaml 协议版本。
const TemplateVersionCurrent = 1

// TemplateYaml 是模板根目录 template.yaml 的解析结果，引擎与模板之间的唯一契约。
type TemplateYaml struct {
	Version   int                      `yaml:"version"`
	Variables map[string]VariableDecl  `yaml:"variables"`
	Patterns  map[string][]ReplaceRule `yaml:"patterns"`
	Init      []string                 `yaml:"init"`
}

// VariableDecl 声明一个需要收集的输入变量。
type VariableDecl struct {
	Prompt   string `yaml:"prompt"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default"`
}

// ReplaceRule 是一条精确字符串替换规则，同时作用于文件路径与内容。
type ReplaceRule struct {
	Pattern string `yaml:"pattern"`
	Replace string `yaml:"replace"`
}

// InitTemplateYaml 解析 template.yaml 内容。未知字段忽略（协议留白字段多，宽容优先），
// yaml 的报错自带行号，直接透传给模板作者。
func InitTemplateYaml(data []byte) (*TemplateYaml, error) {
	var tpl TemplateYaml
	if err := yaml.Unmarshal(data, &tpl); err != nil {
		return nil, fmt.Errorf("template.yaml 解析失败: %w", err)
	}
	if tpl.Version == 0 {
		return nil, fmt.Errorf("template.yaml 缺少 version 字段（当前协议版本为 %d）", TemplateVersionCurrent)
	}
	if tpl.Version > TemplateVersionCurrent {
		return nil, fmt.Errorf("template.yaml 版本 %d 高于引擎支持的版本 %d，请升级 cube", tpl.Version, TemplateVersionCurrent)
	}
	return &tpl, nil
}
