package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"cube/util/iconkit"
)

// ScanRule 扫描规则
type ScanRule struct {
	Group    string        `json:"group"`          // 扫描出的项目组名
	Path     string        `json:"path"`           // 扫描的根目录
	MaxDepth int           `json:"maxDepth"`       // 扫描的最大深度
	Icon     *iconkit.Icon `json:"icon,omitempty"` // 组图标（可选，语义见 util/iconkit）
}

// 项目标签。scanner 命中特征时打标。
//
// 前提假设：所有项目都是 git 项目（.git 存在是项目判定的必要条件），
// 因此不再打通用的 "git" tag——它是冗余信息。标签只标记额外特征：
//   - godot：含 .godot 文件（godot 引擎项目，同时仍是 git 项目）
//
// 注：worktree 目录（.git 为文件）不是项目（1032 归并为项目打开目标），
// 历史的 worktree tag 已随归并移除。

const (
	TagGodot = "godot"
)

func scan(rules []ScanRule) ([]*Project, error) {
	var all []*Project
	for _, rule := range rules {
		got, err := scanOne(rule)
		if err != nil {
			return nil, err
		}
		all = append(all, got...)
	}
	return all, nil
}

func scanOne(r ScanRule) ([]*Project, error) {
	root, maxDepth := r.Path, r.MaxDepth

	var projects []*Project
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		// 检查目录，返回此目录为项目或跳过目录或nil
		isProject, tags, checkErr := checkProjectPath(path)
		if checkErr != nil {
			return checkErr
		}
		if isProject {
			projects = append(projects, newProject(r, path, tags))
			return fs.SkipDir
		}

		// 检查深度
		var depth = 0
		if path != root {
			depth = strings.Count(path[len(root)-1:], "/")
		}
		if depth >= maxDepth {
			return fs.SkipDir
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return projects, nil
}

// skipDirName 判断目录名是否被扫描跳过（以 "." 或 "_" 开头）。
func skipDirName(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func checkProjectPath(path string) (isProject bool, tags []string, err error) {
	// 跳过特殊前缀的目录
	if skipDirName(filepath.Base(path)) {
		return false, nil, fs.SkipDir
	}

	// 获取子文件/子目录用于判断是否是项目及对应 tag
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return false, nil, err
	}
	hasGit := false
	for _, entry := range dirEntries {
		if entry.Name() == ".git" {
			if !entry.IsDir() {
				// .git 是文件 → linked worktree：不是项目（1032 归并为所属主项目的打开目标），
				// 其 git worktree 的可见性来自 git 主项目的 worktree 枚举而非扫描；
				// worktree 子目录也不可能再含主仓库，直接跳过
				return false, nil, fs.SkipDir
			}
			hasGit = true
		} else if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".godot") {
			// godot 标签：在已是 git 项目的前提下，额外标记 godot 引擎项目
			tags = append(tags, TagGodot)
		}
	}
	// 项目判定：必须含 .git（前提：所有项目都是 git 项目）
	if hasGit {
		return true, tags, nil
	}
	return false, nil, nil
}

// MatchScanRule 判断 absPath（git init 后）能否被 scan 收录为新项目，
// 返回匹配的规则及对应项目名。多条规则命中时取根路径最长的一条（最内层规则），
// 与 MatchCloneRule 的最长前缀取舍一致。
//
// 判定条件与 scanOne 的遍历语义一致：
//   - absPath 位于规则根目录之下（或即根目录本身），相对深度 <= maxDepth
//   - 从规则根（含）到 absPath 的每一级目录名均不以 "." 或 "_" 开头——
//     否则遍历在上级就返回 SkipDir，永远不会到达 absPath
func MatchScanRule(absPath string, rules []ScanRule) (rule ScanRule, name string, ok bool) {
	for _, r := range rules {
		rel, err := filepath.Rel(r.Path, absPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue // absPath 不在该规则根目录之下
		}

		// 深度检查：rel == "." 表示 absPath 即规则根自身（深度 0）
		depth := 0
		if rel != "." {
			depth = strings.Count(rel, string(filepath.Separator)) + 1
		}
		if depth > r.MaxDepth {
			continue
		}

		// 前缀检查：规则根到 absPath 的每一级目录都不得以 "." / "_" 开头
		if skipDirName(filepath.Base(r.Path)) || relHasSkipDirName(rel) {
			continue
		}

		if !ok || len(r.Path) > len(rule.Path) {
			rule, name, ok = r, projectName(r, absPath), true
		}
	}
	return
}

// relHasSkipDirName 判断相对路径 rel 的任一级目录名是否被扫描跳过。
// rel 为 "."（路径即规则根，无中间层级）时返回 false。
func relHasSkipDirName(rel string) bool {
	if rel == "." {
		return false
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if skipDirName(part) {
			return true
		}
	}
	return false
}
