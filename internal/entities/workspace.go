package entities

import (
	"github.com/heyuuu/go-cube/internal/config"
	"github.com/heyuuu/go-cube/internal/util/pathkit"
)

// Workspace 工作区
type Workspace struct {
	name       string         // 工作区名，唯一标识
	path       string         // 工作区根目录，唯一索引
	preferApps []string       // 倾向的app列表
	scanner    ProjectScanner // 自动扫描规则
}

func NewWorkspace(conf config.WorkspaceConfig) *Workspace {
	scanner := NewGitProjectScanner(conf.MaxDepth)

	return &Workspace{
		name:       conf.Name,
		path:       pathkit.RealPath(conf.Path),
		preferApps: conf.PreferApps,
		scanner:    scanner,
	}
}

func (ws *Workspace) Name() string            { return ws.name }
func (ws *Workspace) Path() string            { return ws.path }
func (ws *Workspace) PreferApps() []string    { return ws.preferApps }
func (ws *Workspace) Scanner() ProjectScanner { return ws.scanner }
