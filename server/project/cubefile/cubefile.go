// Package cubefile 提供项目根 `.cube/cube.json` 的文件格式层（提案 1030）。
//
// cube.json 是「项目自身的 cube 配置」的容器：进 git 跟仓库走、跨机器共享。
// 当前只有 workspace 相关两节（workspaces / workspaceScanRule），但文件语义
// 不止 workspace——未来任何项目级声明（scan 调优等）都长在这里。因此：
//   - 本包只管文件格式（读写 + 节的存在性语义 + 未知节保留），不 interpret 各节；
//   - 各节的业务语义归各自 domain 包（如 workspace 解析在 project/workspace）。
package cubefile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const cubeFileName = ".cube/cube.json"

// Declared workspaces 节里的单条显式声明（schema 类型；运行期值对象见 workspace.Workspace）。
type Declared struct {
	Name string `json:"name"` // 显示名
	Path string `json:"path"` // 相对所属根的路径；支持 glob 通配（apps/*、packages/**，语义同 pnpm-workspace.yaml 的 packages）
}

// File cube.json 的内存表示。
type File struct {
	Workspaces        []Declared `json:"workspaces,omitempty"`        // 显式声明；WorkspacesSet 区分「字段不存在」与「空声明」
	WorkspacesSet     bool       `json:"-"`                           // workspaces 字段是否存在（空数组也是显式声明：没有任何 workspace，不回落探测）
	WorkspaceScanRule string     `json:"workspaceScanRule,omitempty"` // 探测规则组合（| 分规则组按序，组内 + 合并去重），仅在 workspaces 字段不存在时生效
}

// UnmarshalJSON 用中间结构的指针字段区分 workspaces 字段存在与否。
func (f *File) UnmarshalJSON(data []byte) error {
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

// Load 读取并解析 root/.cube/cube.json；文件不存在或坏 JSON 返回 (nil, false)
// （坏 JSON 属可恢复降级，由调用方决定降级语义——workspace 场景等价无声明走探测）。
func Load(root string) (*File, bool) {
	data, err := os.ReadFile(filepath.Join(root, ".cube", "cube.json"))
	if err != nil {
		return nil, false
	}
	var f File
	if err := f.UnmarshalJSON(data); err != nil {
		return nil, false
	}
	return &f, true
}

// Save 写入 root/.cube/cube.json：File 的已知节整体覆盖，**未知节原样保留**——
// cube.json 是多节容器，本包只认识其中几节，不能把不认识的节写丢。
// workspaces 为空清单时字段仍写出（空数组是有效声明，不能被 omitempty 吞掉）。
func Save(root string, f *File) error {
	raw, err := os.ReadFile(filepath.Join(root, cubeFileName))
	var merged map[string]json.RawMessage
	if err == nil && json.Unmarshal(raw, &merged) == nil {
		for _, k := range knownKeys {
			delete(merged, k)
		}
	} else {
		merged = map[string]json.RawMessage{}
	}
	// 已知节显式组装（不走 File 的 omitempty tag，保留「空数组声明」语义）
	if f.WorkspacesSet {
		if v, err := json.Marshal(f.Workspaces); err == nil {
			merged["workspaces"] = v
		}
	}
	if f.WorkspaceScanRule != "" {
		if v, err := json.Marshal(f.WorkspaceScanRule); err == nil {
			merged["workspaceScanRule"] = v
		}
	}
	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 cube.json 失败: %w", err)
	}
	dir := filepath.Join(root, ".cube")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建 .cube 目录失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cube.json"), append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入 cube.json 失败: %w", err)
	}
	return nil
}

// knownKeys File 当前认识的节；Save 覆盖前先摘掉，未知节留给 merge 保留。
var knownKeys = []string{"workspaces", "workspaceScanRule"}
