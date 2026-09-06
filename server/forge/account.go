package forge

import (
	"fmt"
	"log/slog"
	"strings"

	"cube/settings"
)

// settings.json 中的 account 域节名。
const accountsSection = "forgeAccounts"

// Account 一条 forge 账号配置，纯 API 凭证（token 只用于调平台 API 拉仓库列表，
// 不参与 git 传输——git 操作走本机凭证）。forgeHost+username 归一化后是唯一键。
type Account struct {
	ForgeHost string `json:"forgeHost"` // 所属 forge host，必须命中已配置 forge
	Username  string `json:"username"`  // 平台用户名
	Token     string `json:"token"`     // API token，可为空（仅公开数据）
}

// TokenMasked token 的掩码展示串：list 端点用返回，save 时提交此值视为「未修改」沿用旧值。
const TokenMasked = "••••••"

// NormalizeUsername 归一化用户名：去首尾空白。保留大小写（部分平台用户名大小写敏感）。
func NormalizeUsername(username string) string { return strings.TrimSpace(username) }

// ValidateAccount 校验一条 account（forgeHost 已归一化），坏数据返回中文错误不落文件。
// forge 必须已配置且 kind 非 generic（generic 无 API，凭证无意义）。
func ValidateAccount(a Account, forges []Forge) error {
	if a.ForgeHost == "" {
		return fmt.Errorf("account 所属 forge host 不得为空")
	}
	if NormalizeUsername(a.Username) == "" {
		return fmt.Errorf("account username 不得为空")
	}
	f := MatchHost(forges, a.ForgeHost)
	if f == nil {
		return fmt.Errorf("account 所属 forge 未配置: %s", a.ForgeHost)
	}
	if f.Kind == KindGeneric {
		return fmt.Errorf("forge %s 是 generic 类型（无 API），不支持配置 account", a.ForgeHost)
	}
	return nil
}

// loadAccounts 读全部 account（坏条目跳过不阻断）。
func loadAccounts(file string) []Account {
	var specs []Account
	settings.LoadSection(file, accountsSection, &specs)

	accounts := make([]Account, 0, len(specs))
	for _, a := range specs {
		a.ForgeHost = NormalizeHost(a.ForgeHost)
		a.Username = NormalizeUsername(a.Username)
		if a.Username == "" {
			slog.Warn("forge account 条目非法，跳过", "forgeHost", a.ForgeHost)
			continue
		}
		accounts = append(accounts, a)
	}
	return accounts
}

// saveAccounts 覆写 accounts 节。
func saveAccounts(file string, accounts []Account) error {
	return settings.SaveSection(file, accountsSection, accounts)
}

// findAccount 按 forgeHost+username 找 account，无匹配返回 nil。
func findAccount(accounts []Account, forgeHost, username string) *Account {
	h, u := NormalizeHost(forgeHost), NormalizeUsername(username)
	for i := range accounts {
		if NormalizeHost(accounts[i].ForgeHost) == h && NormalizeUsername(accounts[i].Username) == u {
			return &accounts[i]
		}
	}
	return nil
}
