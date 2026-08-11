package opener

import (
	"cube/config"
	"cube/util/fuzzy"
	"cube/util/slicekit"
)

type Service struct {
	openers []*Opener
}

// NewService 从配置构造 Service。executor 可选，缺省装 NewDefaultExecutor()；测试传 fake。
func NewService(cfg []config.OpenerConfig, executor Executor) *Service {
	var openers []*Opener
	for _, oc := range cfg {
		o, err := InitOpener(oc, executor)
		if err != nil {
			// 解析失败的 opener 跳过（配置错误不阻断启动，list 等命令仍可用）
			continue
		}
		openers = append(openers, o)
	}

	return &Service{openers: openers}
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
	for _, o := range s.openers {
		if o.Name() == name {
			return o
		}
	}
	return nil
}
