package workspace

import (
	"fmt"

	"cube/project/cubefile"
)

// EffectiveScanRule 返回根下当前生效的探测规则（cube.json 的 workspaceScanRule；
// 无声明/字段缺失返回空串，由 Detect 回落 DefaultScanRule）。
// 供 workspace init 等交互入口用「与采集一致的规则」探测。
func EffectiveScanRule(root string) string {
	if f, ok := cubefile.Load(root); ok && !f.WorkspacesSet {
		return f.WorkspaceScanRule
	}
	return ""
}

// Save 把显式 workspaces 声明固化写入 root/.cube/cube.json。
// 声明由此固化为「人的声明」——workspaceScanRule 节随之清掉
// （workspaces 优先生效，探测规则留着只会误导）；其余未知节由 cubefile 原样保留。
// 需至少一条声明（init 的语义就是固化，空清单应直接删文件）。
func Save(root string, ws []Workspace) error {
	if len(ws) == 0 {
		return fmt.Errorf("workspaces 为空，如需清除 workspace 声明请删除 .cube/cube.json: root=%s", root)
	}
	declared := make([]cubefile.Declared, 0, len(ws))
	for _, w := range ws {
		declared = append(declared, cubefile.Declared{Name: w.Name, Path: w.Path})
	}
	return cubefile.Save(root, &cubefile.File{Workspaces: declared, WorkspacesSet: true})
}
