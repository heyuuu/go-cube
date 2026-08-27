package project

import (
	"log/slog"
	"os"

	"cube/settings"
	"cube/util/pathkit"
)

// settingsSection settings.json 中 project 域的节名（scan/clone 规则数据源）。
const settingsSection = "project"

// SettingsSpec settings.json `project` 节的 DTO。
// scan/clone 规则的存储形态与领域形态同构（同一 struct）：存储时 Path/localPath
// 可写 ~/，转换为生效规则时由 makeScanRules/makeCloneRules 展开+校验。
type SettingsSpec struct {
	Scan  []ScanRule  `json:"scan"`
	Clone []CloneRule `json:"clone"`
}

// loadSettingsSpec 现读 settings.json 的 project 节（直读不缓存，改完即生效）。
// settings 包已把文件级/节级坏数据降级为零值。
func loadSettingsSpec(settingsFile string) SettingsSpec {
	var spec SettingsSpec
	settings.LoadSection(settingsFile, settingsSection, &spec)
	return spec
}

// makeScanRules 展开规则路径的 ~/ 为绝对路径，校验目录存在
// （不存在的规则降级跳过、打日志，不阻断其它规则）。
func makeScanRules(specs []ScanRule) []ScanRule {
	var rules []ScanRule
	for _, r := range specs {
		absPath, err := pathkit.StaticAbsPath(r.Path)
		if err != nil {
			slog.Warn("scan 规则路径配置错误，跳过", "group", r.Group, "path", r.Path, "err", err)
			continue
		}
		if info, err := os.Stat(absPath); err != nil || !info.IsDir() {
			slog.Warn("scan 规则路径不存在或非目录，跳过", "group", r.Group, "path", r.Path, "abs", absPath, "err", err)
			continue
		}
		rules = append(rules, ScanRule{Group: r.Group, Path: absPath, MaxDepth: r.MaxDepth})
	}
	return rules
}

// makeCloneRules 展开 localPath 的 ~/ 为绝对路径（不校验存在——clone 时会自动创建）；
// 相对路径是配置错误，跳过。
func makeCloneRules(specs []CloneRule) []CloneRule {
	var rules []CloneRule
	for _, r := range specs {
		absLocalPath, err := pathkit.StaticAbsPath(r.LocalPath)
		if err != nil {
			slog.Warn("clone 本地路径配置错误", "localPath", r.LocalPath, "err", err)
			continue
		}
		rules = append(rules, CloneRule{RepoHost: r.RepoHost, RepoPrefix: r.RepoPrefix, LocalPath: absLocalPath})
	}
	return rules
}
