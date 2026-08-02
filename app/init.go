package app

import (
	"path/filepath"
	"sync"

	"github.com/heyuuu/cube/config"
	"github.com/heyuuu/cube/db"
	"github.com/heyuuu/cube/history"
	"github.com/heyuuu/cube/opener"
	"github.com/heyuuu/cube/project"
	"github.com/heyuuu/cube/web"
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

	configHandler := web.NewConfigHandler(conf)

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
