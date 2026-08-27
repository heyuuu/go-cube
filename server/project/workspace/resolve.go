package workspace

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
	if cf, ok := LoadCubeFile(root); ok {
		if cf.WorkspacesSet {
			return ValidateDeclared(root, cf.Workspaces)
		}
		return Detect(root, cf.WorkspaceScanRule)
	}
	return Detect(root, "")
}

// LoadCubeFile 读取并解析 root/.cube/cube.json；文件不存在或坏 JSON 都按「无声明」处理
// （坏 JSON 属可恢复降级：记日志后走探测，不让一个坏文件拖垮整个项目的采集）。
func LoadCubeFile(root string) (*CubeFile, bool) {
	data, err := os.ReadFile(filepath.Join(root, ".cube", "cube.json"))
	if err != nil {
		return nil, false
	}
	var cf CubeFile
	if err := cf.UnmarshalJSON(data); err != nil {
		slog.Warn("cube.json 解析失败，按无声明处理", "root", root, "err", err)
		return nil, false
	}
	return &cf, true
}

// ValidateDeclared 校验显式声明并构造成员列表：路径须相对、不逃逸出根、目录存在；
// 坏条目跳过（记日志），name 缺省取路径末段。
func ValidateDeclared(root string, declared []Declared) []Workspace {
	result := make([]Workspace, 0, len(declared))
	for _, d := range declared {
		w, ok := validateOne(root, d)
		if !ok {
			continue
		}
		result = append(result, w)
	}
	return result
}

func validateOne(root string, d Declared) (Workspace, bool) {
	if d.Path == "" {
		slog.Warn("cube.json workspace 条目缺 path，跳过", "root", root, "name", d.Name)
		return Workspace{}, false
	}
	if filepath.IsAbs(d.Path) {
		slog.Warn("cube.json workspace 条目 path 须为相对路径，跳过", "root", root, "path", d.Path)
		return Workspace{}, false
	}
	rel := filepath.Clean(d.Path)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		slog.Warn("cube.json workspace 条目 path 逃逸出项目根，跳过", "root", root, "path", d.Path)
		return Workspace{}, false
	}
	info, err := os.Stat(filepath.Join(root, rel))
	if err != nil || !info.IsDir() {
		slog.Warn("cube.json workspace 条目目录不存在，跳过", "root", root, "path", d.Path)
		return Workspace{}, false
	}
	name := d.Name
	if name == "" {
		name = filepath.Base(rel)
	}
	return Workspace{Name: name, Path: rel}, true
}
