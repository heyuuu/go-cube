package forge

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"cube/settings"
	"cube/util/gitapi"
	"cube/util/iconkit"
)

// settings.json 中的 forge 域节名。
const forgesSection = "forges"

// clientFactory 构造平台 API 客户端的工厂签名（测试注入 fake 用）。
type clientFactory func(kind, host, token string) (gitapi.Client, error)

// Service forge 配置管理：settings.json forges / forgeAccounts / forgeNamespaces 三节的
// 读写（直读不缓存，保存即生效），以及 namespace 远端拉取的编排（结果落盘 cacheFile，
// 内存为一级缓存，写穿；见 persist.go）。
type Service struct {
	settingsFile string
	cacheFile    string // 拉取结果落盘文件（cache/forge-repos.json）；空 = 不落盘（测试）

	cacheMu sync.Mutex
	mem     map[string]NsCacheEntry // nsCacheKey(host/path) → 快照（内存一级缓存）
	loaded  bool                    // mem 是否已从落盘文件加载过（懒加载一次）

	newClient clientFactory // 可注入（测试）；nil 用 gitapi.NewClient
}

func NewService(settingsFile, cacheFile string) *Service {
	return &Service{settingsFile: settingsFile, cacheFile: cacheFile}
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

// ReorderForges 按 hosts 顺序重排 forges 节（顺序即展示序，1042 forge 页沿用）。
// 未列出的条目保持原相对顺序排在末尾，不丢数据；未知或重复 host 返回中文错误。
func (s *Service) ReorderForges(hosts []string) error {
	specs := s.Forges()
	byHost := make(map[string]Forge, len(specs))
	for _, spec := range specs {
		h := NormalizeHost(spec.Host)
		if _, dup := byHost[h]; dup {
			return fmt.Errorf("settings.json 存在重复 host 的 forge，无法重排")
		}
		byHost[h] = spec
	}
	seen := make(map[string]bool, len(hosts))
	for _, raw := range hosts {
		h := NormalizeHost(raw)
		if _, ok := byHost[h]; !ok {
			return fmt.Errorf("未找到指定 forge: %s", h)
		}
		if seen[h] {
			return fmt.Errorf("重排名单存在重复 forge: %s", h)
		}
		seen[h] = true
	}

	ordered := make([]Forge, 0, len(specs))
	for _, raw := range hosts {
		ordered = append(ordered, byHost[NormalizeHost(raw)])
	}
	for _, spec := range specs {
		if !seen[NormalizeHost(spec.Host)] {
			ordered = append(ordered, spec)
		}
	}
	return settings.SaveSection(s.settingsFile, forgesSection, ordered)
}

// DeleteForge 按 host 删除一条 forge，并级联清理该 host 下的 account 与 namespace
// （孤挂配置对任何功能都不可见，留了就是脏数据）；不存在时返回中文错误。
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
	if err := settings.SaveSection(s.settingsFile, forgesSection, rest); err != nil {
		return err
	}
	if err := s.retainAccounts(func(a Account) bool { return NormalizeHost(a.ForgeHost) != host }); err != nil {
		return err
	}
	return s.retainNamespaces(func(ns Namespace) bool { return NormalizeHost(ns.ForgeHost) != host })
}

// Accounts 读全部 account（含 token 原文，供拉取取凭证；对外展示的打码在出口层做）。
func (s *Service) Accounts() []Account { return loadAccounts(s.settingsFile) }

// SaveAccount 新增或按 forgeHost+username 替换一条 account。
// token 提交掩码值（TokenMasked）视为「未修改」沿用旧值——前端表单掩码展示的配套语义。
func (s *Service) SaveAccount(a Account) error {
	a.ForgeHost = NormalizeHost(a.ForgeHost)
	a.Username = NormalizeUsername(a.Username)
	if a.Token == TokenMasked {
		existing := findAccount(loadAccounts(s.settingsFile), a.ForgeHost, a.Username)
		if existing == nil {
			return fmt.Errorf("account %s@%s 不存在，token 不能提交掩码占位值", a.Username, a.ForgeHost)
		}
		a.Token = existing.Token
	}
	if err := ValidateAccount(a, s.Forges()); err != nil {
		return err
	}
	accounts := loadAccounts(s.settingsFile)
	replaced := false
	for i, cur := range accounts {
		if NormalizeHost(cur.ForgeHost) == a.ForgeHost && NormalizeUsername(cur.Username) == a.Username {
			accounts[i] = a
			replaced = true
			break
		}
	}
	if !replaced {
		accounts = append(accounts, a)
	}
	return saveAccounts(s.settingsFile, accounts)
}

// DeleteAccount 按 forgeHost+username 删除一条 account；不存在时返回中文错误。
func (s *Service) DeleteAccount(forgeHost, username string) error {
	forgeHost, username = NormalizeHost(forgeHost), NormalizeUsername(username)
	accounts := loadAccounts(s.settingsFile)
	rest := make([]Account, 0, len(accounts))
	for _, cur := range accounts {
		if NormalizeHost(cur.ForgeHost) != forgeHost || NormalizeUsername(cur.Username) != username {
			rest = append(rest, cur)
		}
	}
	if len(rest) == len(accounts) {
		return fmt.Errorf("未找到指定 account: %s@%s", username, forgeHost)
	}
	return saveAccounts(s.settingsFile, rest)
}

// Namespaces 读全部 namespace。
func (s *Service) Namespaces() []Namespace { return loadNamespaces(s.settingsFile) }

// SaveNamespace 新增或按 forgeHost+path 替换一条 namespace。
func (s *Service) SaveNamespace(ns Namespace) error {
	ns.ForgeHost = NormalizeHost(ns.ForgeHost)
	ns.Path = NormalizeNsPath(ns.Path)
	ns.AccountUsername = NormalizeUsername(ns.AccountUsername)
	if err := ValidateNamespace(ns, s.Forges(), s.Accounts()); err != nil {
		return err
	}
	namespaces := loadNamespaces(s.settingsFile)
	replaced := false
	for i, cur := range namespaces {
		if NormalizeHost(cur.ForgeHost) == ns.ForgeHost && strings.EqualFold(cur.Path, ns.Path) {
			namespaces[i] = ns
			replaced = true
			break
		}
	}
	if !replaced {
		namespaces = append(namespaces, ns)
	}
	return saveNamespaces(s.settingsFile, namespaces)
}

// DeleteNamespace 按 forgeHost+path 删除一条 namespace；不存在时返回中文错误。
func (s *Service) DeleteNamespace(forgeHost, path string) error {
	forgeHost = NormalizeHost(forgeHost)
	namespaces := loadNamespaces(s.settingsFile)
	rest := make([]Namespace, 0, len(namespaces))
	for _, cur := range namespaces {
		if NormalizeHost(cur.ForgeHost) != forgeHost || !strings.EqualFold(cur.Path, path) {
			rest = append(rest, cur)
		}
	}
	if len(rest) == len(namespaces) {
		return fmt.Errorf("未找到指定 namespace: %s@%s", path, forgeHost)
	}
	return saveNamespaces(s.settingsFile, rest)
}

// FetchNamespace 拉取 namespace 下的远端仓库列表（force=true 跳过缓存强制重拉）。
// 出站 API 调用是慢操作，手动/低频触发；成功结果写穿内存缓存与落盘文件（含 fetchedAt），
// 失败直接上抛（调用方展示错误，不影响既有缓存）。
func (s *Service) FetchNamespace(forgeHost, path string, force bool) ([]gitapi.RemoteRepo, error) {
	ns := findNamespace(loadNamespaces(s.settingsFile), forgeHost, path)
	if ns == nil {
		return nil, fmt.Errorf("未找到指定 namespace: %s@%s", NormalizeNsPath(path), NormalizeHost(forgeHost))
	}
	key := nsCacheKey(ns.ForgeHost, ns.Path)
	if !force {
		if entry, ok := s.cachedEntry(key); ok {
			return entry.Repos, nil
		}
	}
	repos, err := s.loadRepos(key)
	if err != nil {
		return nil, err
	}
	s.storeEntry(key, NsCacheEntry{FetchedAt: time.Now(), Repos: repos})
	return repos, nil
}

// CachedNamespaceRepos 读 namespace 的拉取快照（含 fetchedAt），从未拉取过返回 false（不触发外呼）。
func (s *Service) CachedNamespaceRepos(forgeHost, path string) (NsCacheEntry, bool) {
	ns := findNamespace(loadNamespaces(s.settingsFile), forgeHost, path)
	if ns == nil {
		return NsCacheEntry{}, false
	}
	return s.cachedEntry(nsCacheKey(ns.ForgeHost, ns.Path))
}

// DetectNamespaceType 探测 path 在该 forge 上是个人空间还是组织空间（探测端点匿名可调）。
func (s *Service) DetectNamespaceType(forgeHost, path string) (gitapi.NamespaceType, error) {
	f := MatchHost(s.Forges(), forgeHost)
	if f == nil {
		return "", fmt.Errorf("未找到指定 forge: %s", NormalizeHost(forgeHost))
	}
	client, err := s.factory()(f.Kind, f.Host, "")
	if err != nil {
		return "", err
	}
	return client.DetectNamespace(context.Background(), NormalizeNsPath(path))
}

// ReconcileNamespace 对账指定 namespace：远端缓存（须先 FetchNamespace）vs 本地仓库快照
// （调用方从 project 领域投影为 LocalRepo 列表，此处按 namespace 范围过滤）。
func (s *Service) ReconcileNamespace(forgeHost, path string, local []LocalRepo) (*ReconcileResult, error) {
	ns := findNamespace(loadNamespaces(s.settingsFile), forgeHost, path)
	if ns == nil {
		return nil, fmt.Errorf("未找到指定 namespace: %s@%s", NormalizeNsPath(path), NormalizeHost(forgeHost))
	}
	entry, ok := s.CachedNamespaceRepos(ns.ForgeHost, ns.Path)
	if !ok {
		return nil, fmt.Errorf("namespace %s@%s 尚未拉取，请先执行拉取", ns.Path, ns.ForgeHost)
	}
	scoped := make([]LocalRepo, 0, len(local))
	for _, l := range local {
		if MatchNamespacePath(l.RepoUrl, ns.ForgeHost, ns.Path) {
			scoped = append(scoped, l)
		}
	}
	result := Reconcile(scoped, entry.Repos)
	return &result, nil
}

// Overview 聚合全部 namespace 的对账行与拉取元信息（forge 页数据源，1042）。
// 只读既有缓存，不触发外呼；未拉取的 namespace 在 Namespaces 元信息中 FetchedAt 为零值。
func (s *Service) Overview(local []LocalRepo) *Overview {
	namespaces := loadNamespaces(s.settingsFile)
	return buildOverview(namespaces, func(ns Namespace) (NsCacheEntry, bool) {
		return s.CachedNamespaceRepos(ns.ForgeHost, ns.Path)
	}, local)
}

// retainAccounts 按谓词保留 account（DeleteForge 级联清理用）。
func (s *Service) retainAccounts(keep func(Account) bool) error {
	accounts := loadAccounts(s.settingsFile)
	rest := make([]Account, 0, len(accounts))
	for _, a := range accounts {
		if keep(a) {
			rest = append(rest, a)
		}
	}
	if len(rest) == len(accounts) {
		return nil
	}
	return saveAccounts(s.settingsFile, rest)
}

// retainNamespaces 按谓词保留 namespace（DeleteForge 级联清理用）。
func (s *Service) retainNamespaces(keep func(Namespace) bool) error {
	namespaces := loadNamespaces(s.settingsFile)
	rest := make([]Namespace, 0, len(namespaces))
	for _, ns := range namespaces {
		if keep(ns) {
			rest = append(rest, ns)
		}
	}
	if len(rest) == len(namespaces) {
		return nil
	}
	return saveNamespaces(s.settingsFile, rest)
}

// factory 返回客户端工厂（未注入时用真实实现，httpClient 默认）。

func (s *Service) factory() clientFactory {
	if s.newClient != nil {
		return s.newClient
	}
	return func(kind, host, token string) (gitapi.Client, error) {
		return gitapi.NewClient(kind, host, token, nil)
	}
}

// loadRepos 按 namespace 键现读配置并执行一次真实拉取（内存缓存未命中时调用）。
func (s *Service) loadRepos(key string) ([]gitapi.RemoteRepo, error) {
	h, p := key2ns(key)
	ns := findNamespace(loadNamespaces(s.settingsFile), h, p)
	if ns == nil {
		return nil, fmt.Errorf("未找到指定 namespace: %s", key)
	}
	f := MatchHost(s.Forges(), ns.ForgeHost)
	if f == nil {
		return nil, fmt.Errorf("namespace 所属 forge 未配置: %s", ns.ForgeHost)
	}
	return fetchRepos(*f, *ns, s.Accounts(), s.factory())
}

// cachedEntry 读内存缓存（懒加载落盘文件一次）。
func (s *Service) cachedEntry(key string) (NsCacheEntry, bool) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.ensureLoadedLocked()
	entry, ok := s.mem[key]
	return entry, ok
}

// storeEntry 更新内存缓存并写穿落盘文件（落盘失败只记日志——缓存语义，下次拉取会重写）。
func (s *Service) storeEntry(key string, entry NsCacheEntry) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.ensureLoadedLocked()
	s.mem[key] = entry
	if s.cacheFile == "" {
		return
	}
	disk := loadDiskCache(s.cacheFile)
	disk.Namespaces[key] = entry
	if err := saveDiskCache(s.cacheFile, disk); err != nil {
		slog.Warn("forge 拉取缓存落盘失败", "file", s.cacheFile, "err", err)
	}
}

// ensureLoadedLocked 首次访问时把落盘文件加载进内存（调用方须持 cacheMu）。
func (s *Service) ensureLoadedLocked() {
	if s.loaded {
		return
	}
	s.loaded = true
	if s.cacheFile == "" {
		s.mem = map[string]NsCacheEntry{}
		return
	}
	s.mem = loadDiskCache(s.cacheFile).Namespaces
}

// nsCacheKey namespace 缓存键。
func nsCacheKey(forgeHost, path string) string {
	return NormalizeHost(forgeHost) + "/" + strings.ToLower(NormalizeNsPath(path))
}

// key2ns 从缓存键反解出 forgeHost 与 path。
func key2ns(key string) (string, string) {
	h, p, _ := strings.Cut(key, "/")
	return h, p
}
