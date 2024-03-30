package project

import (
	"go-cube/internal/matcher"
	"go-cube/internal/slicekit"
	"slices"
	"strings"
)

type Manager struct {
	workspaces []Workspace
}

func NewManager(workspaces []Workspace) *Manager {
	return &Manager{workspaces: workspaces}
}

func (m *Manager) Workspaces() []Workspace {
	return m.workspaces
}

func (m *Manager) Projects() []*Project {
	projectsGroup := slicekit.Map(m.workspaces, Workspace.Projects)
	return slices.Concat(projectsGroup...)
}

func (m *Manager) FindWorkspace(name string) Workspace {
	idx := slices.IndexFunc(m.workspaces, func(ws Workspace) bool {
		return ws.Name() == name
	})
	if idx < 0 {
		return nil
	}

	return m.workspaces[idx]
}

func (m *Manager) FindWorkspaceByProjectName(projectName string) Workspace {
	if wsName, _, ok := strings.Cut(projectName, ":"); ok {
		return m.FindWorkspace(wsName)
	}
	return nil
}

func (m *Manager) Search(query string) []*Project {
	return m.SearchInWorkspace(query, "")
}

func (m *Manager) SearchInWorkspace(query string, workspaceName string) []*Project {
	projects := m.projectsInWorkspace(workspaceName)
	if len(projects) == 0 {
		return nil
	}

	if len(query) == 0 {
		return projects
	}

	projectMatcher := matcher.NewKeywordMatcher(projects, (*Project).Name, nil)
	return projectMatcher.Match(query)
}

func (m *Manager) projectsInWorkspace(workspaceName string) []*Project {
	if workspaceName == "" {
		return m.Projects()
	} else {
		ws := m.FindWorkspace(workspaceName)
		if ws == nil {
			return nil
		}

		return ws.Projects()
	}
}
