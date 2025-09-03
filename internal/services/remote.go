package services

import (
	"github.com/heyuuu/go-cube/internal/config"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
)

type RemoteService struct {
	remotes []*entities.Remote
}

func NewRemoteService() *RemoteService {
	hubConf := config.Default().Repositories.Hubs
	remotes := slicekit.Map(hubConf, func(c config.HubConfig) *entities.Remote {
		return entities.NewHub(c.Name, c.Host, c.DefaultPath)
	})

	return &RemoteService{
		remotes: remotes,
	}
}

func (s *RemoteService) Remotes() []*entities.Remote {
	return s.remotes
}

func (s *RemoteService) FindHubByHost(host string) *entities.Remote {
	for _, hub := range s.remotes {
		if hub.Host() == host {
			return hub
		}
	}
	return nil
}
