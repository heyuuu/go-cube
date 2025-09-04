package services

import (
	"github.com/heyuuu/go-cube/internal/config"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
)

type RemoteService struct {
	remotes []*entities.Remote
}

func NewRemoteService(conf config.Config) *RemoteService {
	remotes := slicekit.Map(conf.Remotes, entities.NewRemote)

	return &RemoteService{
		remotes: remotes,
	}
}

func (s *RemoteService) Remotes() []*entities.Remote {
	return s.remotes
}

func (s *RemoteService) FindByName(name string) *entities.Remote {
	for _, r := range s.remotes {
		if r.Name() == name {
			return r
		}
	}
	return nil
}

func (s *RemoteService) FindByHost(host string) *entities.Remote {
	for _, r := range s.remotes {
		if r.Host() == host {
			return r
		}
	}
	return nil
}
