package repo

import (
	"go-cube/internal/config"
	"go-cube/internal/slicekit"
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
	hubConf := config.Default().Repositories.Hubs
	hubs := slicekit.Map(hubConf, func(c config.HubConfig) *Hub {
		return NewHub(c.Name, c.Host)
	})

	defaultManager = NewManager(hubs)
}
