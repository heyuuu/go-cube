package services

import (
	"github.com/heyuuu/go-cube/internal/config"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
	"strings"
)

type WorkspaceService struct {
	workspaces []*entities.Workspace
}

func NewWorkspaceService(conf config.Config) *WorkspaceService {
	workspaces := slicekit.Map(conf.Workspaces, entities.NewWorkspace)

	return &WorkspaceService{
		workspaces: workspaces,
	}
}

func (s *WorkspaceService) Workspaces() []*entities.Workspace {
	return s.workspaces
}

func (s *WorkspaceService) FindByName(name string) *entities.Workspace {
	for _, ws := range s.workspaces {
		if ws.Name() == name {
			return ws
		}
	}
	return nil
}

func (s *WorkspaceService) FindByProjectName(projectName string) *entities.Workspace {
	if wsName, _, ok := strings.Cut(projectName, ":"); ok {
		return s.FindByName(wsName)
	}
	return nil
}
