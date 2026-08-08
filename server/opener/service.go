package opener

import (
	"sync"

	"cube/config"
	"cube/util/fuzzy"
	"cube/util/slicekit"
)

type Service struct {
	mu      sync.RWMutex
	openers []*Opener
}

func NewService(conf *config.Config) *Service {
	s := &Service{}
	s.applyConf(*conf)
	return s
}

// applyConf 按配置重置 openers 列表。调用方负责持锁（构造时无并发竞争）。
func (s *Service) applyConf(conf config.Config) {
	openers := make([]*Opener, 0, len(conf.Openers))
	for _, oc := range conf.Openers {
		o, err := InitOpener(oc)
		if err != nil {
			// 解析失败的 opener 跳过（配置错误不阻断启动，list 等命令仍可用）
			continue
		}
		openers = append(openers, o)
	}
	s.openers = openers
}

// Reload 用新配置热更新 openers。供配置监听器在 config 变更后调用，
// 让长驻进程（web server）无需重启即可应用新 opener 配置。
func (s *Service) Reload(conf config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyConf(conf)
}

func (s *Service) AllOpeners() []*Opener {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.openers
}

func (s *Service) RoleOpeners(role Role) []*Opener {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slicekit.Filter(s.openers, func(o *Opener) bool {
		return o.HasRole(role)
	})
}

func (s *Service) SearchAll(query string) []*Opener {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fuzzy.MatchBy(query, s.openers, (*Opener).Name, nil)
}

func (s *Service) SearchFor(role Role, query string) []*Opener {
	return fuzzy.MatchBy(query, s.RoleOpeners(role), (*Opener).Name, nil)
}

func (s *Service) FindByName(name string) *Opener {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, app := range s.openers {
		if app.Name() == name {
			return app
		}
	}
	return nil
}
