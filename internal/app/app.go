package app

import "go-cube/internal/matcher"

type App struct {
	Name string
	Bin  string
}

type Manager struct {
	apps    []App
	matcher *matcher.Matcher[App]
}

func NewManager(apps []App) *Manager {
	return &Manager{
		apps:    apps,
		matcher: matcher.NewKeywordMatcher(apps, func(app App) string { return app.Name }, nil),
	}
}

func (m *Manager) Apps() []App {
	return m.apps
}

func (m *Manager) Search(query string) []App {
	return m.matcher.Match(query)
}
