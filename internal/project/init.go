package project

import (
	"go-cube/internal/config"
	"sync"
)

var defaultManager *Manager
var defaultLock sync.Mutex

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
	for i, wsConf := range workspacesConf {
		workspaces[i] = NewDirWorkspace(wsConf.Name, config.RealPath(wsConf.Path), wsConf.MaxDepth, GitProjectChecker)
	}

	defaultManager = NewManager(workspaces)
}
