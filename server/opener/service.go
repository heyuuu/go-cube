package opener

import (
	"cube/config"
	"cube/util/fuzzy"
	"cube/util/slicekit"
)

type Service struct {
	openers []*Opener
}

func NewService(conf *config.Config) *Service {
	s := &Service{}
	s.applyConf(*conf)
	return s
}

// applyConf 按配置重置 openers 列表。
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

func (s *Service) AllOpeners() []*Opener { return s.openers }

func (s *Service) RoleOpeners(role Role) []*Opener {
	return slicekit.Filter(s.openers, func(o *Opener) bool {
		return o.HasRole(role)
	})
}

func (s *Service) SearchAll(query string) []*Opener {
	return fuzzy.MatchBy(query, s.openers, (*Opener).Name, nil)
}

func (s *Service) SearchFor(role Role, query string) []*Opener {
	return fuzzy.MatchBy(query, s.RoleOpeners(role), (*Opener).Name, nil)
}

func (s *Service) FindByName(name string) *Opener {
	for _, app := range s.openers {
		if app.Name() == name {
			return app
		}
	}
	return nil
}
