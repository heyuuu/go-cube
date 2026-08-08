package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ScanRule 扫描规则
type ScanRule struct {
	Group    string `json:"group"`    // 扫描出的项目组名
	Path     string `json:"path"`     // 扫描的根目录
	MaxDepth int    `json:"maxDepth"` // 扫描的最大深度
}

// 项目标签。scanner 命中特征时打标。
//
// 前提假设：所有项目都是 git 项目（.git 存在是项目判定的必要条件），
// 因此不再打通用的 "git" tag——它是冗余信息。标签只标记额外特征：
//   - worktree：.git 是文件而非目录（git worktree，主仓库在别处）
//   - godot：含 .godot 文件（godot 引擎项目，同时仍是 git 项目）
const (
	TagWorktree = "worktree"
	TagGodot    = "godot"
)

func scanProjects(r ScanRule, yield func(path string, tags []string)) error {
	root, maxDepth := r.Path, r.MaxDepth
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
		} else if isProject {
			yield(path, tags)
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
}

func checkProjectPath(path string) (isProject bool, tags []string, err error) {
	// 跳过特殊前缀的目录
	var name = filepath.Base(path)
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
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
			// .git 存在即认为是 git 项目（前提：所有项目都是 git 项目）：
			//   - .git 是目录 → 常规仓库
			//   - .git 是文件 → git worktree（内容形如 "gitdir: <主仓库>/.git/worktrees/<名>"）
			hasGit = true
			if !entry.IsDir() {
				tags = append(tags, TagWorktree)
			}
		} else if hasGit && !entry.IsDir() && strings.HasSuffix(entry.Name(), ".godot") {
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
