//go:build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/heyuuu/go-cube/internal/services"
)

func InitApp() *App {
	wire.Build(
		// services
		services.NewProjectService,
		services.NewApplicationService,
		services.NewRemoteService,

		// app
		wire.Struct(new(App), "*"),
	)
	return nil
}
