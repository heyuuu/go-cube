package entities

type Application struct {
	name string
	bin  string
}

func NewApp(name string, bin string) *Application {
	return &Application{name: name, bin: bin}
}

func (app Application) Name() string { return app.name }
func (app Application) Bin() string  { return app.bin }
