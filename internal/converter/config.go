package converter

import (
	"github.com/heyuuu/go-cube/internal/config"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/pathkit"
)

func ToWorkspaceEntity(conf config.WorkspaceConfig) entities.Workspace {
	return entities.NewDirWorkspace(conf.Name, pathkit.RealPath(conf.Path), conf.MaxDepth, conf.PreferApps)
}

func ToRemoteEntity(conf config.RemoteConfig) *entities.Remote {
	return entities.NewHub(conf.Name, conf.Host, conf.DefaultPath)
}

func ToApplicationEntity(conf config.ApplicationConfig) *entities.Application {
	return entities.NewApp(conf.Name, conf.Bin)
}
