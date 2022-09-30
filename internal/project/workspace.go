package project

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	PathNeedSkip  = errors.New("cube.project: Path need skip")
	PathIsProject = errors.New("cube.project: Path is project")

	GitProjectChecker pathChecker = func(path string) error {
		// 跳过特殊前缀的目录
		var name = filepath.Base(path)
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			return PathNeedSkip
		}

		// 判断若 .git 目录存在则认为是一个 project
		var gitPath = path + "/.git"
		stat, err := os.Stat(gitPath)
		if err == nil && stat.IsDir() {
			return PathIsProject
		}

		return nil
	}
)

// Workspace 项目空间
type Workspace interface {
	Name() string
	Projects() []Project
	Scan() error
}

// DirWorkspace 通过目录管理的项目空间
type DirWorkspace struct {
	name        string
	root        string
	maxDepth    int
	pathChecker pathChecker // 用于判断目录是否为项目或是否应跳过
	scanned     bool
	scanLock    sync.Mutex
	projects    []Project
	files       []string
}

type pathChecker func(string) error

func NewDirWorkspace(name string, root string, maxDepth int, pathChecker pathChecker) *DirWorkspace {
	return &DirWorkspace{
		name:        name,
		root:        root,
		maxDepth:    maxDepth,
		pathChecker: pathChecker,
	}
}

func (s *DirWorkspace) Name() string {
	return s.name
}

func (s *DirWorkspace) Projects() []Project {
	err := s.Scan()
	if err != nil {
		log.Fatal(err)
	}

	return s.projects
}

func (s *DirWorkspace) Scan() error {
	// 已扫描直接返回
	if s.scanned {
		return nil
	}

	// 获取锁
	s.scanLock.Lock()
	defer s.scanLock.Unlock()

	// 获取锁后重新判断是否已扫描(避免获取锁阶段其他goroutine已扫描成功)
	if s.scanned {
		return nil
	}

	var projects []Project
	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		// 检查目录，返回此目录为项目或跳过目录或nil
		err2 := s.pathChecker(path)
		//fmt.Println("err2=", err2)
		if err2 == PathIsProject {
			projects = append(projects, NewProject(s.name+":"+d.Name(), path))
			return fs.SkipDir
		} else if err2 == PathNeedSkip {
			return fs.SkipDir
		} else if err != nil {
			return err
		}

		// 检查深度
		var depth = 0
		if path != s.root {
			depth = strings.Count(path[len(s.root)-1:], "/")
		}
		//fmt.Println("root=", s.root, "path=", path, "depth=", depth)
		if depth >= s.maxDepth {
			return fs.SkipDir
		}

		return nil
	})
	if err != nil {
		return err
	}

	s.projects = projects

	return nil
}

// PathChecker 用于判断目录是否为项目或是否应跳过
type PathChecker interface {
	Check(string, fs.DirEntry, int) error
}
