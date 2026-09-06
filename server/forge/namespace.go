package forge

import (
	"fmt"
	"log/slog"
	"strings"

	"cube/settings"
	"cube/util/gitapi"
)

// settings.json 中的 namespace 域节名。
const namespacesSection = "forgeNamespaces"

// NamespaceType namespace 类型的包内别名（底层定义在 util/gitapi）。
type NamespaceType = gitapi.NamespaceType

// Namespace forge 下的一个仓库命名空间（个人空间 / org 空间，即 URL 的 path 前缀）。
// forgeHost+path 是唯一键；可选挂载一个 account 用于拉取其下私有仓库（正交关系：
// 不挂也能拉公开数据，type 决定拉取端点 personal vs org）。
type Namespace struct {
	ForgeHost       string        `json:"forgeHost"`       // 所属 forge host，必须命中已配置 forge
	Path            string        `json:"path"`            // 命名空间路径，如 heyuuu（保留原样大小写）
	Type            NamespaceType `json:"type"`            // personal / org
	AccountUsername string        `json:"accountUsername"` // 可选，拉取用的 account 用户名（须命中该 forge 下已配置 account）
}

// NormalizeNsPath 归一化 namespace path：去首尾空白与首尾 /（大小写保留——部分自建平台路径大小写敏感）。
func NormalizeNsPath(path string) string {
	return strings.Trim(strings.TrimSpace(path), "/")
}

// ValidateNamespace 校验一条 namespace，坏数据返回中文错误不落文件。
func ValidateNamespace(ns Namespace, forges []Forge, accounts []Account) error {
	if ns.ForgeHost == "" {
		return fmt.Errorf("namespace 所属 forge host 不得为空")
	}
	if NormalizeNsPath(ns.Path) == "" {
		return fmt.Errorf("namespace path 不得为空")
	}
	if !gitapi.ValidNamespaceType(ns.Type) {
		return fmt.Errorf("namespace type 未知: %q（合法值：personal / org）", ns.Type)
	}
	f := MatchHost(forges, ns.ForgeHost)
	if f == nil {
		return fmt.Errorf("namespace 所属 forge 未配置: %s", ns.ForgeHost)
	}
	if f.Kind == KindGeneric {
		return fmt.Errorf("forge %s 是 generic 类型（无 API），不支持配置 namespace", ns.ForgeHost)
	}
	if account := NormalizeUsername(ns.AccountUsername); account != "" && findAccount(accounts, ns.ForgeHost, account) == nil {
		return fmt.Errorf("namespace 挂载的 account 未配置: %s@%s", account, ns.ForgeHost)
	}
	return nil
}

// loadNamespaces 读全部 namespace（坏条目跳过不阻断）。
func loadNamespaces(file string) []Namespace {
	var specs []Namespace
	settings.LoadSection(file, namespacesSection, &specs)

	namespaces := make([]Namespace, 0, len(specs))
	for _, ns := range specs {
		ns.ForgeHost = NormalizeHost(ns.ForgeHost)
		ns.Path = NormalizeNsPath(ns.Path)
		ns.AccountUsername = NormalizeUsername(ns.AccountUsername)
		if ns.Path == "" || !gitapi.ValidNamespaceType(ns.Type) {
			slog.Warn("forge namespace 条目非法，跳过", "forgeHost", ns.ForgeHost, "path", ns.Path, "type", ns.Type)
			continue
		}
		namespaces = append(namespaces, ns)
	}
	return namespaces
}

// saveNamespaces 覆写 namespaces 节。
func saveNamespaces(file string, namespaces []Namespace) error {
	return settings.SaveSection(file, namespacesSection, namespaces)
}

// findNamespace 按 forgeHost+path 找 namespace（path 小写比较），无匹配返回 nil。
func findNamespace(namespaces []Namespace, forgeHost, path string) *Namespace {
	h, p := NormalizeHost(forgeHost), strings.ToLower(NormalizeNsPath(path))
	for i := range namespaces {
		if NormalizeHost(namespaces[i].ForgeHost) == h && strings.ToLower(namespaces[i].Path) == p {
			return &namespaces[i]
		}
	}
	return nil
}
