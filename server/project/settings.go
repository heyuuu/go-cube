package project

import (
	"log/slog"
	"os"

	"cube/settings"
	"cube/util/pathkit"
)

// settings.json 中 project 域的两个规则节：分节存储，可独立读取与写入。
const (
	scanRuleSection  = "scanRule"
	cloneRuleSection = "cloneRule"
)

// loadScanRules 现读 settings.json 的 scanRule 节并转换为生效规则（直读不缓存，改完即生效）：
// 展开 ~/ 为绝对路径，校验目录存在（不存在的规则降级跳过、打日志，不阻断其它规则）。
// settings 包已把文件级/节级坏数据降级为零值。
func loadScanRules(settingsFile string) []ScanRule {
	var specs []ScanRule
	settings.LoadSection(settingsFile, scanRuleSection, &specs)

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

// loadCloneRules 现读 settings.json 的 cloneRule 节并转换为生效规则（直读不缓存，改完即生效）：
// 展开 localPath 的 ~/ 为绝对路径（不校验存在——clone 时会自动创建）；相对路径是配置错误，跳过。
func loadCloneRules(settingsFile string) []CloneRule {
	var specs []CloneRule
	settings.LoadSection(settingsFile, cloneRuleSection, &specs)

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
