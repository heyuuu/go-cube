package workspace

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"cube/project/cubefile"
)

// Resolve 解析项目根（或 worktree 根）的 workspace 列表，返回相对该根的成员目录。
//
// 优先级（提案 1030 讨论定稿）：
//  1. cube.json 存在且 workspaces 字段存在 → 只用它（空数组也是显式声明，不回落探测；
//     条目逐个校验，坏的跳过，全坏等价于空）；
//  2. 否则探测：cube.json 的 workspaceScanRule（无则 DefaultScanRule）按序第一个命中生效；
//  3. cube.json 不存在或坏 JSON → 等价于无声明，走探测。
//
// 本函数供采集侧调用（读文件系统），读路径不应调用——读 projcache 快照。
func Resolve(root string) []Workspace {
	if f, ok := cubefile.Load(root); ok {
		if f.WorkspacesSet {
			return ValidateDeclared(root, f.Workspaces)
		}
		return Detect(root, f.WorkspaceScanRule)
	}
	return Detect(root, "")
}

// ValidateDeclared 校验显式声明并构造成员列表：路径须相对、不逃逸出根、目录存在；
// 坏条目跳过（记日志），name 缺省取路径末段。
//
// path 支持 glob 通配（apps/*、packages/** 等，语义与 pnpm-workspace.yaml 的
// packages 一致：doublestar 展开、排除隐藏与 node_modules 目录）——这是「cube.json
// 完整替代 pnpm-workspace.yaml」的保底表达力。通配命中多目录时各成员取名路径末段
// （name 只对单命中生效）。
func ValidateDeclared(root string, declared []cubefile.Declared) []Workspace {
	result := make([]Workspace, 0, len(declared))
	for _, d := range declared {
		rel, ok := cleanDeclaredPath(root, d.Path)
		if !ok {
			continue
		}
		if hasGlobMeta(rel) {
			result = append(result, expandDeclared(root, d, rel)...)
			continue
		}
		info, err := os.Stat(filepath.Join(root, rel))
		if err != nil || !info.IsDir() {
			slog.Warn("cube.json workspace 条目目录不存在，跳过", "root", root, "path", d.Path)
			continue
		}
		result = append(result, Workspace{Name: declaredName(d.Name, rel), Path: rel})
	}
	return result
}

// cleanDeclaredPath 条目 path 的公共校验：非空、相对、不逃逸出根。
// 返回清理后的相对路径；坏路径返回 ok=false（已记日志）。
func cleanDeclaredPath(root, path string) (string, bool) {
	if path == "" {
		slog.Warn("cube.json workspace 条目缺 path，跳过", "root", root)
		return "", false
	}
	if filepath.IsAbs(path) {
		slog.Warn("cube.json workspace 条目 path 须为相对路径，跳过", "root", root, "path", path)
		return "", false
	}
	rel := filepath.Clean(path)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		slog.Warn("cube.json workspace 条目 path 逃逸出项目根，跳过", "root", root, "path", path)
		return "", false
	}
	return rel, true
}

// hasGlobMeta 判断是否含 glob 通配符（进入通配展开分支而非单目录直查）。
func hasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

// expandDeclared 展开通配声明的命中目录（复用探测侧的 glob 语义）。
// 单命中且声明了 name 时用声明的 name，否则各成员取路径末段。
func expandDeclared(root string, d cubefile.Declared, rel string) []Workspace {
	members := expandPatterns(root, []string{rel})
	if len(members) == 0 {
		slog.Warn("cube.json workspace 通配条目无命中，跳过", "root", root, "path", d.Path)
		return nil
	}
	result := make([]Workspace, 0, len(members))
	for _, m := range members {
		name := filepath.Base(m)
		if len(members) == 1 && d.Name != "" {
			name = d.Name
		}
		result = append(result, Workspace{Name: name, Path: m})
	}
	return result
}

// declaredName 显式条目的显示名：声明了 name 用声明值，缺省取路径末段。
func declaredName(name, rel string) string {
	if name == "" {
		return filepath.Base(rel)
	}
	return name
}
