package project

import (
	"github.com/spf13/viper"
	"log"
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

	// 读取配置
	config := projectConfig{}
	err := viper.UnmarshalKey("project", &config)
	if err != nil {
		log.Fatal(err)
	}

	// 初始化 manager
	var workspaces []Workspace
	for wsName, wsConfig := range config.Workspaces {
		workspaces = append(workspaces, NewDirWorkspace(wsName, wsConfig.Path, wsConfig.MaxDepth, GitProjectChecker))
	}

	defaultManager = NewManager(workspaces)
}
