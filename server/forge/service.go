package forge

import (
	"fmt"
	"log/slog"
	"strings"

	"cube/settings"
	"cube/util/iconkit"
)

// settings.json 中的 forge 域节名。
const forgesSection = "forges"

// Service forge 配置管理：settings.json forges 节的读写（直读不缓存，保存即生效）。
type Service struct {
	settingsFile string
}

func NewService(settingsFile string) *Service {
	return &Service{settingsFile: settingsFile}
}

// Forges 读全部 forge（直读不缓存）。坏条目（host 空 / kind 未知 / icon 非法）跳过不阻断。
func (s *Service) Forges() []Forge {
	var specs []Forge
	settings.LoadSection(s.settingsFile, forgesSection, &specs)

	forges := make([]Forge, 0, len(specs))
	for _, f := range specs {
		f.Host = NormalizeHost(f.Host)
		if err := ValidateHost(f.Host); err != nil || !ValidKind(f.Kind) {
			slog.Warn("forge 配置条目非法，跳过", "host", f.Host, "kind", f.Kind)
			continue
		}
		if err := iconkit.ValidateIcon(f.Icon); err != nil {
			slog.Warn("forge 配置 icon 非法，跳过该条目", "host", f.Host, "err", err)
			continue
		}
		forges = append(forges, f)
	}
	return forges
}

// SaveForge 新增或按 host 替换一条 forge（host 归一化后是唯一键——
// 编辑 host 等价于删旧存新，前端按此语义提交）。校验收敛在写侧，坏数据中文错误不落文件。
func (s *Service) SaveForge(f Forge) error {
	f.Host = NormalizeHost(f.Host)
	if err := ValidateHost(f.Host); err != nil {
		return err
	}
	if !ValidKind(f.Kind) {
		return fmt.Errorf("forge kind 未知: %q（合法值：%s）", f.Kind, strings.Join(Kinds(), " / "))
	}
	if err := iconkit.ValidateIcon(f.Icon); err != nil {
		return fmt.Errorf("forge icon 配置错误: %s", err)
	}

	specs := s.Forges()
	replaced := false
	for i, cur := range specs {
		if NormalizeHost(cur.Host) == f.Host {
			specs[i] = f
			replaced = true
			break
		}
	}
	if !replaced {
		specs = append(specs, f)
	}
	return settings.SaveSection(s.settingsFile, forgesSection, specs)
}

// DeleteForge 按 host 删除一条 forge；不存在时返回中文错误。
func (s *Service) DeleteForge(host string) error {
	host = NormalizeHost(host)
	specs := s.Forges()
	rest := make([]Forge, 0, len(specs))
	for _, cur := range specs {
		if NormalizeHost(cur.Host) != host {
			rest = append(rest, cur)
		}
	}
	if len(rest) == len(specs) {
		return fmt.Errorf("未找到指定 forge: %s", host)
	}
	return settings.SaveSection(s.settingsFile, forgesSection, rest)
}
