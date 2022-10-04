package command

import (
	"github.com/spf13/viper"
	"log"
	"sync"
)

var defaultManager *Manager
var defaultLock sync.Mutex

type applicationConfig map[string]struct{ Bin string }

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
	config := applicationConfig{}
	err := viper.UnmarshalKey("application", &config)
	if err != nil {
		log.Fatal(err)
	}

	// 初始化 manager
	var commands []Command
	for cmdName, cmdConfig := range config {
		commands = append(commands, Command{Name: cmdName, Bin: cmdConfig.Bin})
	}

	defaultManager = NewManager(commands)
}
