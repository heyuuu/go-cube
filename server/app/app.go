package app

import (
	"path/filepath"

	"cube/config"
	"cube/db"
	"cube/history"
	"cube/opener"
	"cube/project"
	"cube/web"
)

type App struct {
	cfg    *config.Config
	server *web.Server

	projectService *project.Service
	openerService  *opener.Service
	historyService *history.Service
}

func New(cfg *config.Config) *App {
	defaultDB := db.Default()

	// ConfigHandler 用 config 包的指针：web 写接口能直接修改 defaultConf 并 Save 持久化
	configHandler := web.NewConfigHandlerPtr(config.DefaultPtr())

	projectService := project.NewService(cfg.Project, filepath.Join(config.Path(), "cache"))

	openerService := opener.NewService(cfg)
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

func (app *App) Config() *config.Config           { return app.cfg }
func (app *App) Server() *web.Server              { return app.server }
func (app *App) ProjectService() *project.Service { return app.projectService }
func (app *App) OpenerService() *opener.Service   { return app.openerService }
func (app *App) HistoryService() *history.Service { return app.historyService }
