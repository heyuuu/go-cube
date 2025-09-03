package services

import "github.com/heyuuu/go-cube/internal/config"

type ConfigService struct {
	conf config.Config
}

func NewConfigService(conf config.Config) *ConfigService {
	return &ConfigService{
		conf: conf,
	}
}

func (s *ConfigService) Config() config.Config {
	return s.conf
}
