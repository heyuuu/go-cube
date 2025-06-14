package services

import (
	"github.com/heyuuu/go-cube/internal/config"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
)

type RepoService struct {
	hubs []*entities.Hub
}

func NewRepoService() *RepoService {
	hubConf := config.Default().Repositories.Hubs
	hubs := slicekit.Map(hubConf, func(c config.HubConfig) *entities.Hub {
		return entities.NewHub(c.Name, c.Host, c.DefaultPath)
	})

	return &RepoService{
		hubs: hubs,
	}
}

func (s *RepoService) Hubs() []*entities.Hub {
	return s.hubs
}

func (s *RepoService) FindHubByHost(host string) *entities.Hub {
	for _, hub := range s.hubs {
		if hub.Host() == host {
			return hub
		}
	}
	return nil
}
