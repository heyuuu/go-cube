package project

import (
	"go-cube/internal/matcher"
	"go-cube/internal/slicekit"
	"slices"
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

func (m *Manager) Search(query string) []*Project {
	projects := m.Projects()
	if len(query) == 0 {
		return projects
	}

	projectMatcher := matcher.NewKeywordMatcher(projects, (*Project).Name, nil)
	return projectMatcher.Match(query)
}

func (m *Manager) SearchInWorkspace(query string, workspaceName string) []*Project {
	ws := m.FindWorkspace(workspaceName)
	if ws == nil {
		return nil
	}

	projects := ws.Projects()
	if len(projects) == 0 {
		return nil
	}

	if len(query) == 0 {
		return projects
	}

	projectMatcher := matcher.NewKeywordMatcher(projects, (*Project).Name, nil)
	return projectMatcher.Match(query)
}
