package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// EffectiveScanRule 返回根下当前生效的探测规则（cube.json 的 workspaceScanRule；
// 无声明/字段缺失返回空串，由 Detect 回落 DefaultScanRule）。
// 供 workspace init 等交互入口用「与采集一致的规则」探测。
func EffectiveScanRule(root string) string {
	if cf, ok := LoadCubeFile(root); ok && !cf.WorkspacesSet {
		return cf.WorkspaceScanRule
	}
	return ""
}

// Save 把显式 workspaces 声明写入 root/.cube/cube.json（覆盖写）。
// 声明由此固化为「人的声明」——探测规则字段不保留（workspaces 优先生效，留着只会误导）。
// 需至少一条声明（init 的语义就是固化，空清单应直接删文件）。
func Save(root string, ws []Workspace) error {
	if len(ws) == 0 {
		return fmt.Errorf("workspaces 为空，如需清除 workspace 声明请删除 .cube/cube.json: root=%s", root)
	}
	declared := make([]Declared, 0, len(ws))
	for _, w := range ws {
		declared = append(declared, Declared{Name: w.Name, Path: w.Path})
	}
	data, err := json.MarshalIndent(CubeFile{Workspaces: declared}, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 cube.json 失败: %w", err)
	}
	dir := filepath.Join(root, ".cube")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建 .cube 目录失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cube.json"), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入 cube.json 失败: %w", err)
	}
	return nil
}
