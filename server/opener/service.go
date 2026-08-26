package opener

import (
	"fmt"
	"slices"

	"cube/settings"
	"cube/util/fuzzy"
	"cube/util/slicekit"
)

// settingsSection settings.json 中 opener 域的节名。
const settingsSection = "openers"

type Service struct {
	settingsFile string   // settings.json 路径，每次查询现读（直读不缓存，Web 改完立刻生效）
	executor     Executor // 逐条构造 execOpener 时注入；nil 时由 InitExecOpener 装默认执行器
}

// NewService 构造 Service。executor 可选；测试传 fake。
func NewService(settingsFile string, executor Executor) *Service {
	return &Service{settingsFile: settingsFile, executor: executor}
}

// openers 现读 settings.json 的 openers 节并逐条构造。
// settings 包已把文件级/节级坏数据降级为空；这里处理条目级：坏条目跳过不阻断
// （配置错误不应让 list 等命令不可用），与旧 config 时代行为一致。
func (s *Service) openers() []Opener {
	var specs []Spec
	settings.LoadSection(s.settingsFile, settingsSection, &specs)

	list := make([]Opener, 0, len(specs))
	for _, spec := range specs {
		o, err := InitExecOpener(spec, s.executor)
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

// SaveOpener 新增或按名替换一条 opener。
// 写前先走领域构造校验（按 Type 分发到对应 Init，只验不留实例）——坏数据返回
// 中文错误、落不了文件（提案「校验收敛在读写边界」的写侧）。
func (s *Service) SaveOpener(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("opener name 不得为空")
	}
	if _, err := InitExecOpener(spec, s.executor); err != nil {
		return err
	}

	var specs []Spec
	settings.LoadSection(s.settingsFile, settingsSection, &specs)
	replaced := false
	for i, cur := range specs {
		if cur.Name == spec.Name {
			specs[i] = spec
			replaced = true
			break
		}
	}
	if !replaced {
		specs = append(specs, spec)
	}
	return settings.SaveSection(s.settingsFile, settingsSection, specs)
}

// DeleteOpener 按名删除一条 opener；不存在时返回中文错误。
func (s *Service) DeleteOpener(name string) error {
	var specs []Spec
	settings.LoadSection(s.settingsFile, settingsSection, &specs)
	rest := make([]Spec, 0, len(specs))
	for _, cur := range specs {
		if cur.Name != name {
			rest = append(rest, cur)
		}
	}
	if len(rest) == len(specs) {
		return fmt.Errorf("未找到指定 opener: %s", name)
	}
	return settings.SaveSection(s.settingsFile, settingsSection, rest)
}
