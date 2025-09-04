package converter

import (
	"github.com/heyuuu/go-cube/internal/config"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/pathkit"
)

func ToWorkspaceEntity(conf config.WorkspaceConfig) *entities.Workspace {
	return entities.NewWorkspace(conf.Name, pathkit.RealPath(conf.Path), conf.MaxDepth, conf.PreferApps)
}

func ToRemoteEntity(conf config.RemoteConfig) *entities.Remote {
	return entities.NewRemote(conf.Name, conf.Host, conf.DefaultPath)
}

func ToApplicationEntity(conf config.ApplicationConfig) *entities.Application {
	return entities.NewApplication(conf.Name, conf.Bin)
}
