package project

import (
	"go-cube/internal/matcher"
)

type Manager struct {
	workspaces []Workspace `get:""`
}

func NewManager(workspaces []Workspace) *Manager {
	return &Manager{workspaces: workspaces}
}

func (m *Manager) Projects() []Project {
	var projects []Project
	for _, workspace := range m.workspaces {
		projects = append(projects, workspace.Projects()...)
	}
	return projects
}

func (m *Manager) Search(query string) []Project {
	if len(query) == 0 {
		return m.Projects()
	}

	return m.projectMatcher().Match(query)
}

func (m *Manager) projectMatcher() *matcher.Matcher[Project] {
	return matcher.NewKeywordMatcher(m.Projects(), func(proj Project) string { return proj.Name }, nil)
}
