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

// 项目标签。scanner 命中特征时打标；用于区分 git/godot 等项目类型。
const (
	TagGit         = "git"
	TagGitWorktree = "git-worktree" // git worktree：.git 是文件而非目录
	TagGodot       = "godot"
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

	// 获取子文件/子目录用于判断是否是项目及对应tag
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return false, nil, err
	}
	for _, entry := range dirEntries {
		if entry.Name() == ".git" {
			// .git 存在即认为是 git 项目：
			//   - .git 是目录 → 常规仓库
			//   - .git 是文件 → git worktree（内容形如 "gitdir: <主仓库>/.git/worktrees/<名>"）
			tags = append(tags, TagGit)
			if !entry.IsDir() {
				tags = append(tags, TagGitWorktree)
			}
		} else if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".godot") {
			tags = append(tags, TagGodot)
		}
	}
	if len(tags) > 0 {
		return true, tags, nil
	}
	return false, nil, nil
}
