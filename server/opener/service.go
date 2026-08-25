package opener

import (
	"slices"

	"cube/settings"
	"cube/util/fuzzy"
	"cube/util/slicekit"
)

// settingsSection settings.json 中 opener 域的节名。
const settingsSection = "openers"

type Service struct {
	settingsFile  string   // settings.json 路径，每次查询现读（直读不缓存，Web 改完立刻生效）
	serverBaseURL string   // cube server 根地址，webOpener 打开浏览器用（如 http://localhost:6001）
	executor      Executor // 逐条构造实现时注入；nil 时由各 Init 装默认执行器
}

// NewService 构造 Service。serverBaseURL 供 web 形态 opener 拼页面地址；executor 可选；测试传 fake。
func NewService(settingsFile, serverBaseURL string, executor Executor) *Service {
	return &Service{
		settingsFile:  settingsFile,
		serverBaseURL: serverBaseURL,
		executor:      executor,
	}
}

// openers 现读 settings.json 的 openers 节，按 Type 分发到对应实现构造。
// settings 包已把文件级/节级坏数据降级为空；这里处理条目级：坏条目跳过不阻断
// （配置错误不应让 list 等命令不可用），与旧 config 时代行为一致。
func (s *Service) openers() []Opener {
	var specs []Spec
	settings.LoadSection(s.settingsFile, settingsSection, &specs)

	list := make([]Opener, 0, len(specs))
	for _, spec := range specs {
		var (
			o   Opener
			err error
		)
		switch spec.Type {
		case SpecTypeExec, "":
			o, err = InitExecOpener(spec, s.executor)
		case SpecTypeWeb:
			o, err = InitWebOpener(spec, s.serverBaseURL, s.executor)
		default:
			continue // 未知 type 视为坏条目
		}
		if err != nil {
			continue
		}
		list = append(list, o)
	}
	return list
}

func (s *Service) AllOpeners() []Opener { return s.openers() }

func (s *Service) RoleOpeners(role Role) []Opener {
	return slicekit.Filter(s.openers(), func(o Opener) bool {
		return slices.Contains(o.Roles(), role)
	})
}

func (s *Service) SearchAll(query string) []Opener {
	return fuzzy.MatchBy(query, s.openers(), Opener.Name, nil)
}

func (s *Service) SearchFor(role Role, query string) []Opener {
	return fuzzy.MatchBy(query, s.RoleOpeners(role), Opener.Name, nil)
}

func (s *Service) FindByName(name string) Opener {
	for _, o := range s.openers() {
		if o.Name() == name {
			return o
		}
	}
	return nil
}
