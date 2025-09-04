package services

import (
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/matcher"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
	"slices"
)

type ProjectService struct {
	workspaceService *WorkspaceService
}

func NewProjectService(workspaceService *WorkspaceService) *ProjectService {
	return &ProjectService{
		workspaceService: workspaceService,
	}
}

func (s *ProjectService) Projects() []*entities.Project {
	workspaces := s.workspaceService.Workspaces()
	projectsGroup := slicekit.Map(workspaces, func(ws *entities.Workspace) []*entities.Project {
		return ws.Projects()
	})
	return slices.Concat(projectsGroup...)
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

		return ws.Projects()
	}
}
