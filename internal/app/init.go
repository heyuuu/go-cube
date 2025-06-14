package app

import "sync"

var (
	defaultApp  *App
	defaultOnce sync.Once
)

func Default() *App {
	defaultOnce.Do(func() {
		defaultApp = InitApp()
	})
	return defaultApp
}
