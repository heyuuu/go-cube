package services

import (
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/matcher"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
	"log"
	"slices"
	"sync"
)

type ProjectService struct {
	workspaceService *WorkspaceService

	scanCache map[string][]*entities.Project
	lockPool  sync.Map
}

func NewProjectService(workspaceService *WorkspaceService) *ProjectService {
	return &ProjectService{
		workspaceService: workspaceService,
		scanCache:        make(map[string][]*entities.Project),
	}
}

func (s *ProjectService) Projects() []*entities.Project {
	workspaces := s.workspaceService.Workspaces()
	projectsGroup := slicekit.Map(workspaces, func(ws *entities.Workspace) []*entities.Project {
		return s.ScanProjects(ws)
	})
	return slices.Concat(projectsGroup...)
}

func (s *ProjectService) FindByName(name string) *entities.Project {
	ws := s.workspaceService.FindByProjectName(name)
	if ws == nil {
		return nil
	}

	for _, project := range s.ScanProjects(ws) {
		if project.Name() == name {
			return project
		}
	}
	return nil
}

func (s *ProjectService) Search(query string) []*entities.Project {
	return s.SearchInWorkspace(query, "")
}

func (s *ProjectService) SearchInWorkspace(query string, workspaceName string) []*entities.Project {
	projects := s.projectsInWorkspace(workspaceName)
	if len(projects) == 0 {
		return nil
	}

	if len(query) == 0 {
		return projects
	}

	projectMatcher := matcher.NewKeywordMatcher(projects, (*entities.Project).Name, nil)
	return projectMatcher.Match(query)
}

func (s *ProjectService) projectsInWorkspace(workspaceName string) []*entities.Project {
	if workspaceName == "" {
		return s.Projects()
	} else {
		ws := s.workspaceService.FindByName(workspaceName)
		if ws == nil {
			return nil
		}

		return s.ScanProjects(ws)
	}
}

func (s *ProjectService) getLock(key string) *sync.RWMutex {
	lock, _ := s.lockPool.LoadOrStore(key, &sync.RWMutex{})
	return lock.(*sync.RWMutex)
}

func (s *ProjectService) ScanProjects(ws *entities.Workspace) []*entities.Project {
	// 判断是否有扫描规则，若没有直接返回
	scanner := ws.Scanner()
	if scanner == nil {
		return nil
	}

	// 获取锁
	lock := s.getLock(ws.Name())

	// 先尝试读缓存
	lock.RLock()
	if projects, ok := s.scanCache[ws.Name()]; ok {
		lock.RUnlock()
		return projects
	}
	lock.RUnlock()

	// 缓存未命中，实际扫描本地文件
	lock.Lock()
	defer lock.Unlock()

	projects, err := scanner.Scan(ws)
	if err != nil {
		log.Print(err)
	}

	// 更新缓存(即使有 err 也更新，避免重复扫描)
	s.scanCache[ws.Name()] = projects
	return projects
}
