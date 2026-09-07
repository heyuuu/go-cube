package opener

import (
	"fmt"
	"log/slog"
	"slices"

	"cube/settings"
	"cube/util/fuzzy"
	"cube/util/slicekit"
)

// settingsSection settings.json 中 opener 域的节名。
const settingsSection = "openers"

// intentsSection settings.json 中打开意图域的节名（1038）：键 = intent，
// 值 = {defaultOpener?, openers?}。openers 节保持纯清单，意图与默认独立成节。
const intentsSection = "openerIntents"

type Service struct {
	settingsFile string   // settings.json 路径，每次查询现读（直读不缓存，Web 改完立刻生效）
	executor     Executor // 逐条构造 actionOpener 时注入；nil 时由 InitActionOpener 装默认执行器
	baseURL      string   // 站内路由基地址（如 http://127.0.0.1:6001），装配注入
}

// NewService 构造 Service。executor 可选（测试传 fake）；baseURL 供 url 动作的
// 站内路由拼接（来自 config 的 server.port，opener 包不 import config，由装配层传值）。
func NewService(settingsFile string, executor Executor, baseURL string) *Service {
	return &Service{settingsFile: settingsFile, executor: executor, baseURL: baseURL}
}

// openers 现读 settings.json 的 openers 节并逐条构造。
// settings 包已把文件级/节级坏数据降级为空；这里处理条目级：坏条目跳过不阻断
// （配置错误不应让 list 等命令不可用），与旧 config 时代行为一致。
func (s *Service) openers() []Opener {
	var specs []Spec
	settings.LoadSection(s.settingsFile, settingsSection, &specs)

	list := make([]Opener, 0, len(specs))
	for _, spec := range specs {
		o, err := InitActionOpener(spec, s.executor, s.baseURL)
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
	if _, err := InitActionOpener(spec, s.executor, s.baseURL); err != nil {
		return err
	}

	var specs []Spec
	settings.LoadSection(s.settingsFile, settingsSection, &specs)
	specs = settings.UpsertKeyed(specs, func(cur Spec) string { return cur.Name }, spec)
	return settings.SaveSection(s.settingsFile, settingsSection, specs)
}

// DeleteOpener 按名删除一条 opener；不存在时返回中文错误。
func (s *Service) DeleteOpener(name string) error {
	var specs []Spec
	settings.LoadSection(s.settingsFile, settingsSection, &specs)
	rest, removed := settings.RemoveKeyed(specs, func(cur Spec) string { return cur.Name }, name)
	if !removed {
		return fmt.Errorf("未找到指定 opener: %s", name)
	}
	if err := settings.SaveSection(s.settingsFile, settingsSection, rest); err != nil {
		return err
	}
	// 连带清理 openerIntents 节对该 opener 的引用（默认 + 候选），避免悬挂引用
	intents := s.loadIntents()
	dirty := false
	for intent, spec := range intents {
		if spec.DefaultOpener == name {
			spec.DefaultOpener = ""
			dirty = true
		}
		if slices.Contains(spec.Openers, name) {
			spec.Openers = slicekit.Filter(spec.Openers, func(n string) bool { return n != name })
			dirty = true
		}
		if spec.DefaultOpener == "" && len(spec.Openers) == 0 {
			delete(intents, intent)
		} else {
			intents[intent] = spec
		}
	}
	if dirty {
		return settings.SaveSection(s.settingsFile, intentsSection, intents)
	}
	return nil
}

// ReorderOpeners 按 names 顺序重排 openers 节。列表顺序即展示顺序（CLI/项目页快捷入口
// 均按此序），前端拖拽排序落库走这里。未列名的条目（含坏条目，list 本就不可见）保持
// 原相对顺序排在末尾，不丢数据；出现未知名或重复名返回中文错误。
func (s *Service) ReorderOpeners(names []string) error {
	var specs []Spec
	settings.LoadSection(s.settingsFile, settingsSection, &specs)
	ordered, err := settings.ReorderKeyed(specs, func(cur Spec) string { return cur.Name }, names, "opener", func(k string) string { return k })
	if err != nil {
		return err
	}
	return settings.SaveSection(s.settingsFile, settingsSection, ordered)
}

// loadIntents 现读 openerIntents 节为 map[Intent]IntentSpec。
// 读侧校验（坏条目跳过 + slog.Warn）：未知 intent 键跳过。
func (s *Service) loadIntents() map[Intent]IntentSpec {
	raw := map[string]IntentSpec{}
	settings.LoadSection(s.settingsFile, intentsSection, &raw)

	out := make(map[Intent]IntentSpec, len(raw))
	for key, spec := range raw {
		intent, err := ParseIntent(key)
		if err != nil {
			slog.Warn("openerIntents 节存在未知 intent，已跳过", "intent", key)
			continue
		}
		out[intent] = spec
	}
	return out
}

// validateIntentOpener 校验 opener 可服务该 intent：存在 + 声明了 intent 对应的 role。
// exists=false 表示 opener 不在 openers 节中。
func validateIntentOpener(intent Intent, o Opener) error {
	role := intent.Role()
	if o == nil {
		return fmt.Errorf("opener 不存在，无法作为 %s 的默认", intent)
	}
	if !slices.Contains(o.Roles(), role) {
		return fmt.Errorf("opener %s 未声明 %s，不能作为 %s 的默认", o.Name(), role, intent)
	}
	return nil
}

// Intents 合成全部意图状态（固定序）：每个 intent 给出默认 opener 与候选清单。
// 读侧双重校验：defaultOpener / openers 成员失效（不存在或 role 不符）时跳过该条并
// slog.Warn；openers 缺省回退为「声明了对应 role 的全部 opener」（按 openers 节的顺序）。
func (s *Service) Intents() []IntentInfo {
	specs := s.loadIntents()
	byName := make(map[string]Opener)
	for _, o := range s.openers() {
		byName[o.Name()] = o
	}

	out := make([]IntentInfo, 0, len(intentOrder))
	for _, intent := range intentOrder {
		spec := specs[intent]
		info := IntentInfo{Intent: intent, Openers: []string{}}

		if spec.DefaultOpener != "" {
			o := byName[spec.DefaultOpener]
			if err := validateIntentOpener(intent, o); err != nil {
				slog.Warn("openerIntents 默认 opener 失效，已忽略", "intent", intent, "opener", spec.DefaultOpener, "err", err)
			} else {
				info.DefaultOpener = spec.DefaultOpener
			}
		}

		if len(spec.Openers) > 0 {
			for _, name := range spec.Openers {
				o := byName[name]
				if err := validateIntentOpener(intent, o); err != nil {
					slog.Warn("openerIntents 候选 opener 失效，已跳过", "intent", intent, "opener", name, "err", err)
					continue
				}
				info.Openers = append(info.Openers, name)
			}
		} else {
			// 缺省候选 = 声明了对应 role 的全部 opener（openers 节顺序）
			for _, o := range s.RoleOpeners(intent.Role()) {
				info.Openers = append(info.Openers, o.Name())
			}
		}
		out = append(out, info)
	}
	return out
}

// DefaultOpener 返回该 intent 的默认 opener；未配置或失效时返回中文错误
// （CLI 不带 -o 时直接走这里，报错即引导用户去配置）。
func (s *Service) DefaultOpener(intent Intent) (Opener, error) {
	if _, err := ParseIntent(string(intent)); err != nil {
		return nil, err
	}
	specs := s.loadIntents()
	spec, ok := specs[intent]
	if !ok || spec.DefaultOpener == "" {
		return nil, fmt.Errorf("intent %s 未配置默认 opener（settings.json 的 openerIntents 节或 Web 设置页配置）", intent)
	}
	o := s.FindByName(spec.DefaultOpener)
	if err := validateIntentOpener(intent, o); err != nil {
		return nil, fmt.Errorf("intent %s 的默认 opener 失效: %w", intent, err)
	}
	return o, nil
}

// SaveIntentDefault 设置某 intent 的默认 opener。写侧校验：intent 合法 + opener
// 存在且声明了对应 role（必死配置不落盘）；其余条目原样保留。
func (s *Service) SaveIntentDefault(intent Intent, openerName string) error {
	if _, err := ParseIntent(string(intent)); err != nil {
		return err
	}
	if err := validateIntentOpener(intent, s.FindByName(openerName)); err != nil {
		return err
	}

	specs := s.loadIntents()
	spec := specs[intent]
	spec.DefaultOpener = openerName
	specs[intent] = spec
	return settings.SaveSection(s.settingsFile, intentsSection, specs)
}

// DeleteIntentDefault 清除某 intent 的默认 opener（候选 openers 清单如有则保留）。
func (s *Service) DeleteIntentDefault(intent Intent) error {
	if _, err := ParseIntent(string(intent)); err != nil {
		return err
	}
	specs := s.loadIntents()
	spec, ok := specs[intent]
	if !ok || spec.DefaultOpener == "" {
		return fmt.Errorf("intent %s 未配置默认 opener", intent)
	}
	spec.DefaultOpener = ""
	if len(spec.Openers) == 0 {
		delete(specs, intent)
	} else {
		specs[intent] = spec
	}
	return settings.SaveSection(s.settingsFile, intentsSection, specs)
}
