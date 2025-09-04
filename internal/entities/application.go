package entities

import "github.com/heyuuu/go-cube/internal/config"

type Application struct {
	name string // 应用名, 唯一标识符
	bin  string // 应用路径
}

func NewApplication(conf config.ApplicationConfig) *Application {
	return &Application{
		name: conf.Name,
		bin:  conf.Bin,
	}
}

func (app *Application) Name() string { return app.name }
func (app *Application) Bin() string  { return app.bin }
