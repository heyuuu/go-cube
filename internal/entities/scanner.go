package entities

import (
	"github.com/heyuuu/go-cube/internal/common"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type ProjectScanner interface {
	Scan(ws *Workspace) (projects []*Project, err error)
}

// GitProjectScanner 扫描 git 项目的规则
type GitProjectScanner struct {
	maxDepth int
}

func NewGitProjectScanner(maxDepth int) *GitProjectScanner {
	return &GitProjectScanner{maxDepth: maxDepth}
}

func (sc *GitProjectScanner) MaxDepth() int {
	return sc.maxDepth
}

func (sc *GitProjectScanner) Scan(ws *Workspace) (projects []*Project, err error) {
	root := ws.Path()
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		// 检查目录，返回此目录为项目或跳过目录或nil
		isProject, tags, checkErr := sc.checkPath(path)
		if checkErr != nil {
			return checkErr
		} else if isProject {
			projects = append(projects, NewProject(ws, path, tags))
			return fs.SkipDir
		}

		// 检查深度
		var depth = 0
		if path != root {
			depth = strings.Count(path[len(root)-1:], "/")
		}
		if depth >= sc.maxDepth {
			return fs.SkipDir
		}

		return nil
	})
	return
}

func (sc *GitProjectScanner) checkPath(path string) (isProject bool, tags []string, err error) {
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
		if entry.IsDir() && entry.Name() == ".git" { // 若 .git 目录存在则认为是一个 project
			tags = append(tags, common.TagGit)
		} else if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".godot") {
			tags = append(tags, common.TagGodot)
		}
	}
	if len(tags) > 0 {
		return true, tags, nil
	}
	return false, nil, nil
}
