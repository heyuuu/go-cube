package project

import (
	"go-cube/internal/matcher"
)

var (
	defaultManager = NewManager(
		[]Workspace{
			NewDirWorkspace("bin", "/Users/heyu/Code/bin", 0, GitProjectChecker),
			NewDirWorkspace("ke", "/Users/heyu/Code/ke", 5, GitProjectChecker),
			NewDirWorkspace("github", "/Users/heyu/Code/github", 2, GitProjectChecker),
			NewDirWorkspace("local", "/Users/heyu/Code/local", 3, GitProjectChecker),
			NewDirWorkspace("temp", "/Users/heyu/Code/temp", 1, GitProjectChecker),
		},
	)
)

func DefaultManager() *Manager {
	return defaultManager
}

type Manager struct {
	workspaces []Workspace
}

func NewManager(workspaces []Workspace) *Manager {
	return &Manager{workspaces: workspaces}
}

func (p *Manager) Projects() []Project {
	var projects []Project
	for _, workspace := range p.workspaces {
		projects = append(projects, workspace.Projects()...)
	}
	return projects
}

func (p *Manager) Search(query string) []Project {
	if len(query) == 0 {
		return p.Projects()
	}

	return p.projectMatcher().Match(query)
}

func (p *Manager) projectMatcher() *matcher.Matcher[Project] {
	return matcher.NewKeywordMatcher(p.Projects(), func(proj Project) string { return proj.Name }, matcher.DefaultScorer)
}
