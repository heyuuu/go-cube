package services

import (
	"github.com/heyuuu/go-cube/internal/config"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/matcher"
	"github.com/heyuuu/go-cube/internal/util/pathkit"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
	"slices"
	"strings"
)

type ProjectService struct {
	workspaces []entities.Workspace
}

func NewProjectService() *ProjectService {
	workspacesConf := config.Default().Workspaces
	workspaces := make([]entities.Workspace, len(workspacesConf))
	for i, wsConf := range workspacesConf {
		workspaces[i] = entities.NewDirWorkspace(wsConf.Name, pathkit.RealPath(wsConf.Path), wsConf.MaxDepth, wsConf.PreferApps)
	}

	return &ProjectService{
		workspaces: workspaces,
	}
}

func (s *ProjectService) Workspaces() []entities.Workspace {
	return s.workspaces
}

func (s *ProjectService) Projects() []*entities.Project {
	projectsGroup := slicekit.Map(s.workspaces, entities.Workspace.Projects)
	return slices.Concat(projectsGroup...)
}

func (s *ProjectService) FindWorkspace(name string) entities.Workspace {
	idx := slices.IndexFunc(s.workspaces, func(ws entities.Workspace) bool {
		return ws.Name() == name
	})
	if idx < 0 {
		return nil
	}

	return s.workspaces[idx]
}

func (s *ProjectService) FindWorkspaceByProjectName(projectName string) entities.Workspace {
	if wsName, _, ok := strings.Cut(projectName, ":"); ok {
		return s.FindWorkspace(wsName)
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
		ws := s.FindWorkspace(workspaceName)
		if ws == nil {
			return nil
		}

		return ws.Projects()
	}
}
