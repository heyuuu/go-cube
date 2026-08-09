package app

import (
	"gorm.io/gorm"

	"cube/config"
	"cube/db"
	"cube/history"
	"cube/opener"
	"cube/project"
	"cube/web"
)

type App struct {
	cfg   *config.Config
	db    *gorm.DB
	paths *Paths

	server *web.Server

	projectService *project.Service
	openerService  *opener.Service
	historyService *history.Service
}

func New(cfg *config.Config) (*App, error) {
	paths := NewPaths(cfg.DataDir)

	// 初始数数据库
	dataDb, err := db.Init(paths.DataDbFile(),
		&history.ProjectSelectLog{},
		&history.ProjectOpenLog{},
	)
	if err != nil {
		return nil, err
	}

	// 组装 services
	projectService := project.NewService(cfg.Project, paths.CacheDir())
	openerService := opener.NewService(cfg)
	historyService := history.NewService(dataDb)

	// 组装 web server
	configHandler := web.NewConfigHandler(cfg)
	projectHandler := web.NewProjectHandler(projectService, openerService)
	openerHandler := web.NewOpenerHandler(openerService)
	server := web.NewServer(
		configHandler,
		projectHandler,
		openerHandler,
	)

	return &App{
		cfg:    cfg,
		db:     dataDb,
		paths:  paths,
		server: server,

		projectService: projectService,
		openerService:  openerService,
		historyService: historyService,
	}, nil
}

func (a *App) Config() *config.Config           { return a.cfg }
func (a *App) Db() *gorm.DB                     { return a.db }
func (a *App) Paths() *Paths                    { return a.paths }
func (a *App) Server() *web.Server              { return a.server }
func (a *App) ProjectService() *project.Service { return a.projectService }
func (a *App) OpenerService() *opener.Service   { return a.openerService }
func (a *App) HistoryService() *history.Service { return a.historyService }
