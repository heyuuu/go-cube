//go:build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/heyuuu/go-cube/internal/config"
	"github.com/heyuuu/go-cube/internal/services"
)

func InitApp() *App {
	wire.Build(
		// config
		config.Default,

		// services
		services.NewProjectService,
		services.NewApplicationService,
		services.NewRemoteService,

		// app
		wire.Struct(new(App), "*"),
	)
	return nil
}
