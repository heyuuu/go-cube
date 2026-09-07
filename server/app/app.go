package app

import (
	"path/filepath"

	"cube/config"
	"cube/create"
	"cube/forge"
	"cube/handlers"
	"cube/opener"
	"cube/project"
	"cube/usage"
	"cube/web"
	"cube/workbench"
)

type App struct {
	cfg   *config.Config
	paths *Paths

	server *web.Server

	// services 有序清单，供生命周期钩子（OnAppCreated / OnServerStart / OnServerStop）分发
	services []any

	projectService   *project.Service
	workbenchService *workbench.Service
	openerService    *opener.Service
	usageService     *usage.Service
	createService    *create.Service
	forgeService     *forge.Service
}

func New(cfg *config.Config) (*App, error) {
	paths := NewPaths(cfg.DataDir)

	// 组装 services
	projectService := project.NewService(paths.SettingsFile(), paths.CacheDir())
	openerService := opener.NewService(paths.SettingsFile(), nil, web.BaseURL(cfg.Server.Port))
	usageService := usage.NewService(paths.UsageFile())
	workbenchService := workbench.NewService(projectService.RefreshGitInfo)
	createService := create.NewService(cfg.Create)
	forgeService := forge.NewService(paths.SettingsFile(), filepath.Join(paths.CacheDir(), "forge-repos.json"))
	services := []any{projectService, openerService, usageService, workbenchService, createService, forgeService}

	// 组装 web server
	configHandler := handlers.NewConfigHandler(cfg)
	projectHandler := handlers.NewProjectHandler(projectService, openerService, usageService)
	openerHandler := handlers.NewOpenerHandler(openerService)
	mdHandler := handlers.NewMdHandler()
	iconHandler := handlers.NewIconHandler()
	workbenchHandler := handlers.NewWorkbenchHandler(workbenchService)
	forgeHandler := handlers.NewForgeHandler(forgeService, projectService)
	usageHandler := handlers.NewUsageHandler(usageService)
	server := web.NewServer(
		cfg.Server,
		[]web.Handler{
			configHandler,
			projectHandler,
			openerHandler,
			mdHandler,
			workbenchHandler,
			forgeHandler,
			usageHandler,
			iconHandler,
		},
	)

	return &App{
		cfg:    cfg,
		paths:  paths,
		server: server,

		services:         services,
		projectService:   projectService,
		workbenchService: workbenchService,
		openerService:    openerService,
		usageService:     usageService,
		createService:    createService,
		forgeService:     forgeService,
	}, nil
}

func (a *App) Config() *config.Config               { return a.cfg }
func (a *App) Paths() *Paths                        { return a.paths }
func (a *App) Server() *web.Server                  { return a.server }
func (a *App) ProjectService() *project.Service     { return a.projectService }
func (a *App) WorkbenchService() *workbench.Service { return a.workbenchService }
func (a *App) OpenerService() *opener.Service       { return a.openerService }
func (a *App) UsageService() *usage.Service         { return a.usageService }
func (a *App) CreateService() *create.Service       { return a.createService }
func (a *App) ForgeService() *forge.Service         { return a.forgeService }

// StartBackgroundJobs 启动常驻进程的后台任务（分发到各 service 的 OnServerStart 钩子）。
// 仅常驻 server 调用；CLI 短命进程不调用。
func (a *App) StartBackgroundJobs() {
	for _, s := range a.services {
		if h, ok := s.(serverStartHook); ok {
			h.OnServerStart()
		}
	}
}

// StopBackgroundJobs 停止后台任务（server 退出时调，分发到 OnServerStop 钩子）。
func (a *App) StopBackgroundJobs() {
	for _, s := range a.services {
		if h, ok := s.(serverStopHook); ok {
			h.OnServerStop()
		}
	}
}
