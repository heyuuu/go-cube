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

	// ConfigHandler 用 config 包的指针：web 写接口能直接修改 defaultConf 并 Save 持久化
	configHandler := web.NewConfigHandler(cfg)

	projectService := project.NewService(cfg.Project, paths.CacheDir())

	openerService := opener.NewService(cfg)
	openerHandler := web.NewOpenerHandler(openerService)

	projectHandler := web.NewProjectHandler(projectService, openerService)

	server := web.NewServer(
		configHandler,
		projectHandler,
		openerHandler,
	)

	historyService := history.NewService(dataDb)

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
