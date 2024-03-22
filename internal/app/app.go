package app

import "go-cube/internal/matcher"

type App struct {
	name string
	bin  string
}

func MakeApp(name string, bin string) App {
	return App{name: name, bin: bin}
}

func (app App) Name() string { return app.name }
func (app App) Bin() string  { return app.bin }

type Manager struct {
	apps    []App
	matcher *matcher.Matcher[App]
}

func NewManager(apps []App) *Manager {
	return &Manager{
		apps:    apps,
		matcher: matcher.NewKeywordMatcher(apps, func(app App) string { return app.name }, nil),
	}
}

func (m *Manager) Apps() []App {
	return m.apps
}

func (m *Manager) Search(query string) []App {
	return m.matcher.Match(query)
}
