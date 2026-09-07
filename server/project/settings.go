package project

import (
	"fmt"
	"log/slog"
	"os"

	"cube/settings"
	"cube/util/iconkit"
	"cube/util/pathkit"
)

// settings.json 中 project 域的两个规则节：分节存储，可独立读取与写入。
const (
	scanRulesSection  = "scanRules"
	cloneRulesSection = "cloneRules"
)

// loadScanRules 现读 settings.json 的 scanRule 节并转换为生效规则（直读不缓存，改完即生效）：
// 展开 ~/ 为绝对路径，校验目录存在（不存在的规则降级跳过、打日志，不阻断其它规则）。
// settings 包已把文件级/节级坏数据降级为零值。
func loadScanRules(settingsFile string) []ScanRule {
	var specs []ScanRule
	settings.LoadSection(settingsFile, scanRulesSection, &specs)

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
		rules = append(rules, ScanRule{Group: r.Group, Path: absPath, MaxDepth: r.MaxDepth, Icon: r.Icon})
	}
	return rules
}

// loadCloneRules 现读 settings.json 的 cloneRule 节并转换为生效规则（直读不缓存，改完即生效）：
// 展开 localPath 的 ~/ 为绝对路径（不校验存在——clone 时会自动创建）；相对路径是配置错误，跳过。
func loadCloneRules(settingsFile string) []CloneRule {
	var specs []CloneRule
	settings.LoadSection(settingsFile, cloneRulesSection, &specs)

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

// --- 写侧（校验收敛在读写边界：加载即校验、校验后才保存，坏数据返回中文错误、落不了文件） ---

// CloneRuleKey clone 规则的唯一键（host + prefix 唯一标识一条规则）。
type CloneRuleKey struct {
	RepoHost   string `json:"repoHost"`
	RepoPrefix string `json:"repoPrefix"`
}

// cloneRuleKey / formatCloneRuleKey clone 规则的节内操作键（settings 三原语用）与错误文案渲染。
func cloneRuleKey(r CloneRule) CloneRuleKey    { return CloneRuleKey{r.RepoHost, r.RepoPrefix} }
func formatCloneRuleKey(k CloneRuleKey) string { return k.RepoHost + k.RepoPrefix }

// saveScanRule 新增或按 path 替换一条 scan 规则（path 原串是规则唯一键——
// 编辑 path 等价于删旧存新，前端按此语义提交）。
func saveScanRule(settingsFile string, rule ScanRule) error {
	if rule.Group == "" {
		return fmt.Errorf("scan 规则 group 不得为空")
	}
	if rule.MaxDepth <= 0 {
		return fmt.Errorf("scan 规则 maxDepth 必须大于 0: %d", rule.MaxDepth)
	}
	absPath, err := pathkit.StaticAbsPath(rule.Path)
	if err != nil {
		return fmt.Errorf("scan 规则路径不合法: %w", err)
	}
	if info, err := os.Stat(absPath); err != nil || !info.IsDir() {
		return fmt.Errorf("scan 规则路径不存在或非目录: %s", rule.Path)
	}
	if err := iconkit.ValidateIcon(rule.Icon); err != nil {
		return fmt.Errorf("scan 规则 %s", err)
	}

	var specs []ScanRule
	settings.LoadSection(settingsFile, scanRulesSection, &specs)
	specs = settings.UpsertKeyed(specs, func(cur ScanRule) string { return cur.Path }, rule)
	return settings.SaveSection(settingsFile, scanRulesSection, specs)
}

// deleteScanRule 按 path 删除一条 scan 规则；不存在时返回中文错误。
func deleteScanRule(settingsFile, path string) error {
	var specs []ScanRule
	settings.LoadSection(settingsFile, scanRulesSection, &specs)
	rest, removed := settings.RemoveKeyed(specs, func(cur ScanRule) string { return cur.Path }, path)
	if !removed {
		return fmt.Errorf("未找到指定 scan 规则: %s", path)
	}
	return settings.SaveSection(settingsFile, scanRulesSection, rest)
}

// reorderScanRules 按 paths 顺序重排 scanRule 节（顺序即项目列表展示序）。
// 未列出的条目保持原相对顺序排在末尾，不丢数据；未知或重复路径返回中文错误。
func reorderScanRules(settingsFile string, paths []string) error {
	var specs []ScanRule
	settings.LoadSection(settingsFile, scanRulesSection, &specs)
	ordered, err := settings.ReorderKeyed(specs, func(cur ScanRule) string { return cur.Path }, paths, "scan 规则", func(k string) string { return k })
	if err != nil {
		return err
	}
	return settings.SaveSection(settingsFile, scanRulesSection, ordered)
}

// saveCloneRule 新增或按 host+prefix 替换一条 clone 规则（二者组合是规则唯一键）。
func saveCloneRule(settingsFile string, rule CloneRule) error {
	if rule.RepoHost == "" {
		return fmt.Errorf("clone 规则 repoHost 不得为空")
	}
	if rule.RepoPrefix != "" && rule.RepoPrefix[0] != '/' {
		return fmt.Errorf("clone 规则 repoPrefix 须以 / 开头或为空: %s", rule.RepoPrefix)
	}
	if _, err := pathkit.StaticAbsPath(rule.LocalPath); err != nil {
		return fmt.Errorf("clone 规则 localPath 不合法（须绝对路径或 ~/ 前缀）: %s", rule.LocalPath)
	}

	var specs []CloneRule
	settings.LoadSection(settingsFile, cloneRulesSection, &specs)
	specs = settings.UpsertKeyed(specs, cloneRuleKey, rule)
	return settings.SaveSection(settingsFile, cloneRulesSection, specs)
}

// deleteCloneRule 按 host+prefix 删除一条 clone 规则；不存在时返回中文错误。
func deleteCloneRule(settingsFile string, key CloneRuleKey) error {
	var specs []CloneRule
	settings.LoadSection(settingsFile, cloneRulesSection, &specs)
	rest, removed := settings.RemoveKeyed(specs, cloneRuleKey, key)
	if !removed {
		return fmt.Errorf("未找到指定 clone 规则: %s%s", key.RepoHost, key.RepoPrefix)
	}
	return settings.SaveSection(settingsFile, cloneRulesSection, rest)
}

// reorderCloneRules 按键顺序重排 cloneRule 节（顺序即展示序；匹配语义按 prefix
// 最长优先，顺序不影响路由结果）。语义约束同 reorderScanRules。
func reorderCloneRules(settingsFile string, keys []CloneRuleKey) error {
	var specs []CloneRule
	settings.LoadSection(settingsFile, cloneRulesSection, &specs)
	ordered, err := settings.ReorderKeyed(specs, cloneRuleKey, keys, "clone 规则", formatCloneRuleKey)
	if err != nil {
		return err
	}
	return settings.SaveSection(settingsFile, cloneRulesSection, ordered)
}
