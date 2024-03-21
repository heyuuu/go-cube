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

func (m *Manager) Search(query string) []*Project {
	projects := m.Projects()
	if len(query) == 0 {
		return projects
	}

	projectMatcher := matcher.NewKeywordMatcher(projects, (*Project).Name, nil)
	return projectMatcher.Match(query)
}
