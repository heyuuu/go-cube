package app

import (
	"cube/history"
	"cube/opener"
	"cube/project"
	"cube/web"
)

type App struct {
	server *web.Server

	projectService *project.Service
	openerService  *opener.Service
	historyService *history.Service
}

func (app *App) Server() *web.Server              { return app.server }
func (app *App) ProjectService() *project.Service { return app.projectService }
func (app *App) OpenerService() *opener.Service   { return app.openerService }
func (app *App) HistoryService() *history.Service { return app.historyService }
