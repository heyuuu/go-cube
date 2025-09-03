package services

import (
	"github.com/heyuuu/go-cube/internal/config"
	"github.com/heyuuu/go-cube/internal/converter"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
)

type RemoteService struct {
	remotes []*entities.Remote
}

func NewRemoteService(conf config.Config) *RemoteService {
	remotes := slicekit.Map(conf.Remotes, converter.ToRemoteEntity)

	return &RemoteService{
		remotes: remotes,
	}
}

func (s *RemoteService) Remotes() []*entities.Remote {
	return s.remotes
}

func (s *RemoteService) FindByHost(host string) *entities.Remote {
	for _, hub := range s.remotes {
		if hub.Host() == host {
			return hub
		}
	}
	return nil
}
