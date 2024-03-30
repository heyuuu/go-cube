package project

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	gitProjectChecker pathChecker = func(path string) (isProject bool, err error) {
		// 跳过特殊前缀的目录
		var name = filepath.Base(path)
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			return false, fs.SkipDir
		}

		// 判断若 .git 目录存在则认为是一个 project
		var gitPath = path + "/.git"
		stat, err := os.Stat(gitPath)
		if err == nil && stat.IsDir() {
			return true, nil
		}

		return false, nil
	}
)

// Workspace 项目空间
type Workspace interface {
	Name() string
	Projects() []*Project
	Scan() error
	PreferApps() []string
}

// DirWorkspace 通过目录管理的项目空间
type DirWorkspace struct {
	name        string      // 工作区名
	root        string      // 根目录
	maxDepth    int         // 扫描最大深度
	preferApps  []string    // 倾向的app列表
	pathChecker pathChecker // 用于判断目录是否为项目或是否应跳过
	scanned     bool        // 是否已扫描
	scanLock    sync.Mutex
	projects    []*Project
}

type pathChecker func(path string) (isProject bool, err error)

func NewDirWorkspace(name string, root string, maxDepth int, preferApps []string) *DirWorkspace {
	return &DirWorkspace{
		name:        name,
		root:        root,
		maxDepth:    maxDepth,
		preferApps:  preferApps,
		pathChecker: gitProjectChecker,
	}
}

func (ws *DirWorkspace) Name() string         { return ws.name }
func (ws *DirWorkspace) Root() string         { return ws.root }
func (ws *DirWorkspace) MaxDepth() int        { return ws.maxDepth }
func (ws *DirWorkspace) PreferApps() []string { return ws.preferApps }

func (ws *DirWorkspace) Projects() []*Project {
	err := ws.Scan()
	if err != nil {
		log.Fatal(err)
	}

	return ws.projects
}

func (ws *DirWorkspace) Scan() error {
	// 已扫描直接返回
	if ws.scanned {
		return nil
	}

	// 获取锁
	ws.scanLock.Lock()
	defer ws.scanLock.Unlock()

	// 获取锁后重新判断是否已扫描(避免获取锁阶段其他goroutine已扫描成功)
	if ws.scanned {
		return nil
	}

	var projects []*Project
	err := filepath.WalkDir(ws.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		// 检查目录，返回此目录为项目或跳过目录或nil
		isProject, checkErr := ws.pathChecker(path)
		if checkErr != nil {
			return checkErr
		} else if isProject {
			name, _ := filepath.Rel(ws.root, path)
			if name == "." {
				name = filepath.Base(path)
			}
			projects = append(projects, NewProject(ws.name+":"+name, path))
			return fs.SkipDir
		}

		// 检查深度
		var depth = 0
		if path != ws.root {
			depth = strings.Count(path[len(ws.root)-1:], "/")
		}
		//fmt.Println("root=", ws.root, "path=", path, "depth=", depth)
		if depth >= ws.maxDepth {
			return fs.SkipDir
		}

		return nil
	})
	if err != nil {
		return err
	}

	ws.scanned = true
	ws.projects = projects

	return nil
}
