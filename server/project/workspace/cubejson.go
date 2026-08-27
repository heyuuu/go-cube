// Package workspace 提供 monorepo workspace 的声明解析与探测（提案 1030）。
//
// 数据来源两级：项目根 .cube/cube.json 的显式声明（人的、跟仓库走）优先；
// 字段缺失时按 workspaceScanRule（或程序默认规则）探测标准 monorepo 声明文件
// （机器推导）。本包只做解析与推导，不落盘——持久化的是声明文件本身，
// 推导结果由 projcache 采集侧带入快照。
package workspace

import (
	"encoding/json"
	"fmt"
)

// Declared cube.json workspaces 节里的单条显式声明。
type Declared struct {
	Name string `json:"name"` // 显示名
	Path string `json:"path"` // 相对所属根的路径
}

// CubeFile .cube/cube.json 的反序列化结构。
type CubeFile struct {
	Workspaces        []Declared `json:"workspaces,omitempty"`        // 显式声明；WorkspacesSet 区分「字段不存在」与「空声明」
	WorkspacesSet     bool       `json:"-"`                           // workspaces 字段是否存在（空数组也是显式声明：没有任何 workspace，不回落探测）
	WorkspaceScanRule string     `json:"workspaceScanRule,omitempty"` // 探测规则组合（逗号分隔按序），仅在 workspaces 字段不存在时生效
}

// UnmarshalJSON 用中间结构的指针字段区分 workspaces 字段存在与否。
func (f *CubeFile) UnmarshalJSON(data []byte) error {
	type alias struct {
		Workspaces        *[]Declared `json:"workspaces,omitempty"`
		WorkspaceScanRule string      `json:"workspaceScanRule,omitempty"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("解析 cube.json 失败: %w", err)
	}
	if a.Workspaces != nil {
		f.Workspaces = *a.Workspaces
		f.WorkspacesSet = true
	}
	f.WorkspaceScanRule = a.WorkspaceScanRule
	return nil
}
