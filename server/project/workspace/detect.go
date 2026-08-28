// Package workspace 提供 monorepo workspace 的解析与探测（提案 1030）。
//
// 数据来源两级：.cube/cube.json 的显式声明（人的、跟仓库走，文件格式见 project/cubefile）
// 优先；字段缺失时按 workspaceScanRule（或程序默认规则）探测标准 monorepo 声明文件
// （机器推导）。本包只做解析与推导——持久化的是声明文件本身，推导结果由 projcache
// 采集侧带入快照。
package workspace

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// Workspace 一个可作为打开入口的子目录（显式声明或探测推导而来）。
// Path 相对所属根（主项目根或 worktree 根），已清理。
type Workspace struct {
	Name string `json:"name"` // 显示名
	Path string `json:"path"` // 相对所属根的路径
}

// DefaultScanRule 默认探测规则：pnpm 最强（pnpm-workspace.yaml 是显式 monorepo 声明），
// npm（package.json workspaces，覆盖 yarn）次之。其余探测器实现了也不默认开——
// 误报率高，想要的人显式写 workspaceScanRule。
const DefaultScanRule = "pnpm|npm"

// detector 探测器：给定根目录，返回相对根的成员目录列表（空 = 未命中）。
type detector func(root string) []string

// detectors 探测器注册表。新增生态在此登记，名字即可被 workspaceScanRule 引用。
var detectors = map[string]detector{
	"pnpm": detectPnpm,
	"npm":  detectNpm,
}

// Detect 按 scanRule 探测根下的 workspace 成员。
// 规则格式："pnpm+npm | go-work"——整体以 | 分割为多个规则组，按序尝试，
// 第一个「合并后至少有一个成员」的规则组生效并返回；组内以 + 分割多个规则，
// 各自探测后合并去重（同一目录被多个来源声明只留一份）。不用逗号做分隔符——
// 逗号表达不了「并列合并」与「按序回落」的区别。rule 为空时用 DefaultScanRule；
// 未知规则名跳过（等价于该规则无命中）。
func Detect(root string, rule string) []Workspace {
	if strings.TrimSpace(rule) == "" {
		rule = DefaultScanRule
	}
	for _, group := range strings.Split(rule, "|") {
		members := detectGroup(root, group)
		if len(members) > 0 {
			return makeWorkspaces(members)
		}
	}
	return nil
}

// detectGroup 跑单个规则组（+ 分割），合并去重各规则的命中结果，按路径排序保持稳定。
func detectGroup(root string, group string) []string {
	var merged []string
	seen := map[string]bool{}
	for _, name := range strings.Split(group, "+") {
		name = strings.TrimSpace(name)
		d, ok := detectors[name]
		if !ok {
			continue
		}
		for _, m := range d(root) {
			if !seen[m] {
				seen[m] = true
				merged = append(merged, m)
			}
		}
	}
	if len(merged) > 0 {
		sort.Strings(merged)
	}
	return merged
}

// makeWorkspaces 把相对目录列表构造成 Workspace（显示名取路径末段），保持输入顺序。
func makeWorkspaces(members []string) []Workspace {
	result := make([]Workspace, 0, len(members))
	for _, m := range members {
		result = append(result, Workspace{Name: filepath.Base(m), Path: m})
	}
	return result
}

// detectPnpm 解析 pnpm-workspace.yaml 的 packages 通配符。
func detectPnpm(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, "pnpm-workspace.yaml"))
	if err != nil {
		return nil
	}
	var spec struct {
		Packages []string `yaml:"packages"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil
	}
	return expandPatterns(root, spec.Packages)
}

// detectNpm 解析 package.json 的 workspaces 节（数组或 {packages: []} 两种形态，yarn 同）。
func detectNpm(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil || len(pkg.Workspaces) == 0 {
		return nil
	}
	var patterns []string
	var asList []string
	if err := json.Unmarshal(pkg.Workspaces, &asList); err == nil {
		patterns = asList
	} else {
		var asObj struct {
			Packages []string `json:"packages"`
		}
		if err := json.Unmarshal(pkg.Workspaces, &asObj); err != nil {
			return nil
		}
		patterns = asObj.Packages
	}
	return expandPatterns(root, patterns)
}

// maxGlobWalkDepth glob 展开的遍历深度上限：workspace 声明不可能埋得很深，
// 防御性截断超大目录树的遍历成本（** 模式递归任意深）。
const maxGlobWalkDepth = 6

// expandPatterns 把相对 glob 模式（apps/*、packages/** 等）展开为根下实际存在的目录列表。
// 结果去重（多模式可重叠）并按路径排序，保证探测输出稳定。
func expandPatterns(root string, patterns []string) []string {
	if len(patterns) == 0 {
		return nil
	}
	rootRel := "."
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 单目录读失败（权限等）跳过，不中断整次探测
		}
		if !d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == rootRel {
			return nil
		}
		depth := strings.Count(rel, string(filepath.Separator))
		if depth >= maxGlobWalkDepth {
			return fs.SkipDir
		}
		if isGlobDir(rel) {
			return fs.SkipDir // node_modules / .git 等不进 glob 结果也不下钻
		}
		for _, p := range patterns {
			if p == "" {
				continue
			}
			// doublestar 的 "**" 可匹配零段，libs/** 会连 libs 本身也命中——
			// 模式自身的静态前缀是「容器目录」不是成员，排除
			if prefix, ok := strings.CutSuffix(filepath.ToSlash(p), "/**"); ok && filepath.ToSlash(rel) == prefix {
				continue
			}
			if ok, _ := doublestar.Match(filepath.ToSlash(p), filepath.ToSlash(rel)); ok {
				dirs = append(dirs, rel)
				break
			}
		}
		return nil
	})
	if len(dirs) == 0 {
		return nil
	}
	sort.Strings(dirs)
	return dirs
}

// isGlobDir glob 展开明确排除的目录名前缀（与 scan 的跳过口径一致）。
func isGlobDir(rel string) bool {
	base := filepath.Base(rel)
	return strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_") || base == "node_modules"
}
