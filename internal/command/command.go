package command

import "go-cube/internal/matcher"

type Command struct {
	Name string
	Bin  string
}

type Manager struct {
	commands []Command
}

func NewManager(commands []Command) *Manager {
	return &Manager{commands: commands}
}

func (m Manager) Commands() []Command {
	return m.commands
}

func (m Manager) Search(query string) []Command {
	if len(query) == 0 {
		return m.commands
	}

	return m.matcher().Match(query)
}

func (m Manager) matcher() *matcher.Matcher[Command] {
	return matcher.NewKeywordMatcher(m.commands, func(c Command) string { return c.Name }, nil)
}
