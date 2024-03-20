package command

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

	// 读取配置
	conf := config.Default().Applications

	// 初始化 manager
	var commands []Command
	for _, app := range conf {
		commands = append(commands, Command{Name: app.Name, Bin: app.Bin})
	}

	defaultManager = NewManager(commands)
}
