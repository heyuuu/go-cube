package app

import (
	"github.com/heyuuu/go-cube/internal/server"
	"github.com/heyuuu/go-cube/internal/services"
)

type App struct {
	server *server.Server

	workspaceService   *services.WorkspaceService
	projectService     *services.ProjectService
	applicationService *services.ApplicationService
	remoteService      *services.RemoteService
}

func (app *App) Server() *server.Server {
	return app.server
}

func (app *App) WorkspaceService() *services.WorkspaceService {
	return app.workspaceService
}

func (app *App) ProjectService() *services.ProjectService {
	return app.projectService
}

func (app *App) ApplicationService() *services.ApplicationService {
	return app.applicationService
}

func (app *App) RemoteService() *services.RemoteService {
	return app.remoteService
}
