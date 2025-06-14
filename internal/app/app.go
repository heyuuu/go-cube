package app

import (
	"github.com/heyuuu/go-cube/internal/services"
)

type App struct {
	projectService     *services.ProjectService
	applicationService *services.ApplicationService
	repoService        *services.RepoService
}

func (app *App) ProjectService() *services.ProjectService {
	return app.projectService
}

func (app *App) ApplicationService() *services.ApplicationService {
	return app.applicationService
}

func (app *App) RepoService() *services.RepoService {
	return app.repoService
}
