package project

import (
	"go-cube/internal/config"
	"sync"
)

var defaultManager *Manager
var defaultLock sync.Mutex

type projectConfig struct {
	Workspaces map[string]struct {
		Path                 string
		MaxDepth             int
		PriorityApplications []string
	}
}

func DefaultManager() *Manager {
	initDefaultManager()
	return defaultManager
}

func initDefaultManager() {
	if defaultManager != nil {
		return
	}

	defaultLock.Lock()
	defer defaultLock.Unlock()

	if defaultManager != nil {
		return
	}

	// 初始化 manager
	workspacesConf := config.Default().Workspaces
	workspaces := make([]Workspace, len(workspacesConf))
	for _, wsConf := range workspacesConf {
		workspaces = append(workspaces, NewDirWorkspace(wsConf.Name, wsConf.Path, wsConf.MaxDepth, GitProjectChecker))
	}

	defaultManager = NewManager(workspaces)
}
