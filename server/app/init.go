package app

import (
	"path/filepath"
	"sync"

	"cube/config"
	"cube/db"
	"cube/history"
	"cube/opener"
	"cube/project"
	"cube/web"
)

var (
	defaultApp  *App
	defaultOnce sync.Once
)

func Default() *App {
	defaultOnce.Do(func() {
		defaultApp = InitApp()
	})
	return defaultApp
}

func InitApp() *App {
	conf := config.Default()
	defaultDB := db.Default()

	// ConfigHandler 用 config 包的指针：web 写接口能直接修改 defaultConf 并 Save 持久化
	configHandler := web.NewConfigHandlerPtr(config.DefaultPtr())

	projectService := project.NewService(conf.Project, filepath.Join(config.Path(), "cache"))

	openerService := opener.NewService(conf)
	openerHandler := web.NewOpenerHandler(openerService)

	projectHandler := web.NewProjectHandler(projectService, openerService)

	server := web.NewServer(
		configHandler,
		projectHandler,
		openerHandler,
	)

	historyService := history.NewService(defaultDB)

	return &App{
		server:         server,
		projectService: projectService,
		openerService:  openerService,
		historyService: historyService,
	}
}
