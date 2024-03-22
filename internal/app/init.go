package app

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

	// 读取配置
	conf := config.Default().Applications

	// 初始化 manager
	apps := slicekit.Map(conf, func(c config.ApplicationConfig) App {
		return MakeApp(c.Name, c.Bin)
	})

	defaultManager = NewManager(apps)
}
