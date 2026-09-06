package forge

import (
	"strings"
	"testing"

	"context"

	"cube/internal/testfixture"
	"cube/settings"
	"cube/util/gitapi"
	"cube/util/iconkit"
)

func newService(t *testing.T) (*Service, *testfixture.Workspace) {
	t.Helper()
	ws := testfixture.NewWorkspace(t)
	return NewService(ws.Join("settings.json"), ws.Join("cache/forge-repos.json")), ws
}

// TestSaveForge 新增与按 host 替换（归一化后同键）。
func TestSaveForge(t *testing.T) {
	s, _ := newService(t)

	if err := s.SaveForge(Forge{Host: " GitHub.COM. ", Kind: KindGithub}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if got := s.Forges(); len(got) != 1 || got[0].Host != "github.com" {
		t.Fatalf("保存后应归一化 host: %v", got)
	}

	// 同 host 替换（改 kind + icon）
	if err := s.SaveForge(Forge{
		Host: "github.com",
		Kind: KindGeneric,
		Icon: &iconkit.Icon{Type: iconkit.IconTypeLucide, Value: "github"},
	}); err != nil {
		t.Fatalf("替换失败: %v", err)
	}
	if got := s.Forges(); len(got) != 1 || got[0].Kind != KindGeneric || got[0].Icon == nil {
		t.Fatalf("同 host 应替换而非追加: %v", got)
	}

	// 不同 host 追加
	if err := s.SaveForge(Forge{Host: "gitee.com", Kind: KindGitee}); err != nil {
		t.Fatalf("追加失败: %v", err)
	}
	if got := s.Forges(); len(got) != 2 {
		t.Fatalf("应有 2 条 forge: %v", got)
	}
}

// TestSaveForge_Validation 坏数据返回中文错误、落不了文件。
func TestSaveForge_Validation(t *testing.T) {
	s, _ := newService(t)

	cases := []struct {
		name string
		f    Forge
		want string
	}{
		{"host 为空", Forge{Kind: KindGithub}, "host"},
		{"host 带协议", Forge{Host: "https://github.com", Kind: KindGithub}, "纯域名"},
		{"kind 未知", Forge{Host: "github.com", Kind: "gitlab"}, "kind"},
		{"icon 非法", Forge{Host: "github.com", Kind: KindGithub, Icon: &iconkit.Icon{Type: "svg", Value: "x"}}, "icon"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.SaveForge(c.f)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("应报含 %q 的中文错误，实际: %v", c.want, err)
			}
			if got := s.Forges(); len(got) != 0 {
				t.Fatalf("坏数据不应落文件，实际: %v", got)
			}
		})
	}
}

// TestDeleteForge 按 host 删除；不存在返回中文错误。
func TestDeleteForge(t *testing.T) {
	s, _ := newService(t)
	if err := s.SaveForge(Forge{Host: "github.com", Kind: KindGithub}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	if err := s.DeleteForge("GITHUB.com"); err != nil {
		t.Fatalf("删除（归一化匹配）失败: %v", err)
	}
	if got := s.Forges(); len(got) != 0 {
		t.Fatalf("删除后应为空: %v", got)
	}

	err := s.DeleteForge("github.com")
	if err == nil || !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("删不存在应报中文错误，实际: %v", err)
	}
}

// TestForges_SkipBadEntries 手写进 settings.json 的坏条目读侧跳过，不阻断其他条目。
func TestForges_SkipBadEntries(t *testing.T) {
	s, _ := newService(t)
	if err := settings.SaveSection(s.settingsFile, forgesSection, []Forge{
		{Host: "github.com", Kind: KindGithub},
		{Host: "", Kind: KindGithub},
		{Host: "gitea.io", Kind: "unknown"},
	}); err != nil {
		t.Fatalf("写入测试数据失败: %v", err)
	}

	got := s.Forges()
	if len(got) != 1 || got[0].Host != "github.com" {
		t.Fatalf("坏条目应跳过，只留合法条目: %v", got)
	}
}

// TestReorderForges 按 host 重排：未列出条目原序殿后、未知/重复报中文错误。
func TestReorderForges(t *testing.T) {
	s, _ := newService(t)
	for _, f := range []Forge{
		{Host: "github.com", Kind: KindGithub},
		{Host: "gitee.com", Kind: KindGitee},
		{Host: "gitea.example.com", Kind: KindGitea},
	} {
		if err := s.SaveForge(f); err != nil {
			t.Fatalf("保存失败: %v", err)
		}
	}

	if err := s.ReorderForges([]string{"gitee.com", "gitea.example.com"}); err != nil {
		t.Fatalf("重排失败: %v", err)
	}
	got := s.Forges()
	if got[0].Host != "gitee.com" || got[1].Host != "gitea.example.com" || got[2].Host != "github.com" {
		t.Fatalf("未列出的 github 应原序殿后: %v", got)
	}

	if err := s.ReorderForges([]string{"unknown.com"}); err == nil || !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("未知 host 应报中文错误, got %v", err)
	}
	if err := s.ReorderForges([]string{"gitee.com", "GITEE.com"}); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("重复 host（归一化后）应报中文错误, got %v", err)
	}
}

// --- account / namespace（1041） ---

// seedForge 写入一个 github.com forge，供 account/namespace 校验命中。
func seedForge(t *testing.T, s *Service) {
	t.Helper()
	if err := s.SaveForge(Forge{Host: "github.com", Kind: KindGithub}); err != nil {
		t.Fatalf("seed forge 失败: %v", err)
	}
}

// TestAccountCrud account 保存/替换/删除 + 校验（forge 未配置 / generic 拒配）。
func TestAccountCrud(t *testing.T) {
	s, _ := newService(t)

	if err := s.SaveAccount(Account{ForgeHost: "github.com", Username: "heyuuu", Token: "t1"}); err == nil {
		t.Fatal("forge 未配置时应报错")
	}
	seedForge(t, s)

	// generic forge 拒配
	if err := s.SaveForge(Forge{Host: "plain.example.com", Kind: KindGeneric}); err != nil {
		t.Fatalf("seed generic forge 失败: %v", err)
	}
	if err := s.SaveAccount(Account{ForgeHost: "plain.example.com", Username: "a"}); err == nil {
		t.Fatal("generic forge 应拒配 account")
	}

	if err := s.SaveAccount(Account{ForgeHost: "GitHub.COM", Username: "heyuuu", Token: "t1"}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	// 同键替换
	if err := s.SaveAccount(Account{ForgeHost: "github.com", Username: "heyuuu", Token: "t2"}); err != nil {
		t.Fatalf("替换失败: %v", err)
	}
	accounts := s.Accounts()
	if len(accounts) != 1 || accounts[0].Token != "t2" || accounts[0].ForgeHost != "github.com" {
		t.Fatalf("account 状态不符: %+v", accounts)
	}

	// 掩码值 = 沿用旧 token；新账号提交掩码值报错
	if err := s.SaveAccount(Account{ForgeHost: "github.com", Username: "heyuuu", Token: TokenMasked}); err != nil {
		t.Fatalf("掩码提交应沿用旧值: %v", err)
	}
	if s.Accounts()[0].Token != "t2" {
		t.Fatalf("掩码提交后 token 不应变: %q", s.Accounts()[0].Token)
	}
	if err := s.SaveAccount(Account{ForgeHost: "github.com", Username: "new", Token: TokenMasked}); err == nil {
		t.Fatal("新账号提交掩码值应报错")
	}

	if err := s.DeleteAccount("github.com", "ghost"); err == nil {
		t.Fatal("删除不存在的 account 应报错")
	}
	if err := s.DeleteAccount("github.com", "heyuuu"); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if len(s.Accounts()) != 0 {
		t.Fatal("删除后应为空")
	}
}

// TestNamespaceCrud namespace 保存/替换/删除 + 校验（type 合法性、account 引用）。
func TestNamespaceCrud(t *testing.T) {
	s, _ := newService(t)
	seedForge(t, s)
	if err := s.SaveAccount(Account{ForgeHost: "github.com", Username: "heyuuu", Token: "t1"}); err != nil {
		t.Fatalf("seed account 失败: %v", err)
	}

	if err := s.SaveNamespace(Namespace{ForgeHost: "github.com", Path: "heyuuu", Type: "team"}); err == nil {
		t.Fatal("未知 type 应报错")
	}
	if err := s.SaveNamespace(Namespace{ForgeHost: "github.com", Path: "heyuuu", Type: gitapi.NamespacePersonal, AccountUsername: "ghost"}); err == nil {
		t.Fatal("挂载未配置 account 应报错")
	}

	if err := s.SaveNamespace(Namespace{ForgeHost: "github.com", Path: "/heyuuu/", Type: gitapi.NamespacePersonal, AccountUsername: "heyuuu"}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	nss := s.Namespaces()
	if len(nss) != 1 || nss[0].Path != "heyuuu" || nss[0].AccountUsername != "heyuuu" {
		t.Fatalf("namespace 状态不符: %+v", nss)
	}
	// 同键（path 归一化后）替换
	if err := s.SaveNamespace(Namespace{ForgeHost: "github.com", Path: "heyuuu", Type: gitapi.NamespaceOrg}); err != nil {
		t.Fatalf("替换失败: %v", err)
	}
	if len(s.Namespaces()) != 1 || s.Namespaces()[0].Type != gitapi.NamespaceOrg {
		t.Fatalf("替换后状态不符: %+v", s.Namespaces())
	}

	if err := s.DeleteNamespace("github.com", "ghost"); err == nil {
		t.Fatal("删除不存在的 namespace 应报错")
	}
	if err := s.DeleteNamespace("github.com", "HEYUUU"); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if len(s.Namespaces()) != 0 {
		t.Fatal("删除后应为空")
	}
}

// TestDeleteForgeCascade 删 forge 级联清理该 host 的 account 与 namespace。
func TestDeleteForgeCascade(t *testing.T) {
	s, _ := newService(t)
	seedForge(t, s)
	if err := s.SaveForge(Forge{Host: "gitea.example.com", Kind: KindGitea}); err != nil {
		t.Fatalf("seed 失败: %v", err)
	}
	mustSave := func(a Account, ns Namespace) {
		if err := s.SaveAccount(a); err != nil {
			t.Fatalf("保存 account 失败: %v", err)
		}
		if err := s.SaveNamespace(ns); err != nil {
			t.Fatalf("保存 namespace 失败: %v", err)
		}
	}
	mustSave(Account{ForgeHost: "github.com", Username: "heyuuu", Token: "t"}, Namespace{ForgeHost: "github.com", Path: "heyuuu", Type: "personal"})
	mustSave(Account{ForgeHost: "gitea.example.com", Username: "ops", Token: "t"}, Namespace{ForgeHost: "gitea.example.com", Path: "acme", Type: "org"})

	if err := s.DeleteForge("github.com"); err != nil {
		t.Fatalf("删 forge 失败: %v", err)
	}
	if len(s.Accounts()) != 1 || s.Accounts()[0].ForgeHost != "gitea.example.com" {
		t.Fatalf("account 应只剩 gitea 一条: %+v", s.Accounts())
	}
	if len(s.Namespaces()) != 1 || s.Namespaces()[0].ForgeHost != "gitea.example.com" {
		t.Fatalf("namespace 应只剩 gitea 一条: %+v", s.Namespaces())
	}
}

// fakeClient 固定返回预置结果（List/Detect 调用计数供断言）。
type fakeClient struct {
	repos  []gitapi.RemoteRepo
	detect gitapi.NamespaceType
	lists  int
}

func (f *fakeClient) ListNamespaceRepos(_ context.Context, _ gitapi.NamespaceType, _ string) ([]gitapi.RemoteRepo, error) {
	f.lists++
	return f.repos, nil
}

func (f *fakeClient) DetectNamespace(_ context.Context, _ string) (gitapi.NamespaceType, error) {
	return f.detect, nil
}

// TestFetchAndReconcile 拉取缓存（命中不重拉）+ namespace 范围对账。
func TestFetchAndReconcile(t *testing.T) {
	s, _ := newService(t)
	seedForge(t, s)
	if err := s.SaveNamespace(Namespace{ForgeHost: "github.com", Path: "heyuuu", Type: "personal"}); err != nil {
		t.Fatalf("保存 namespace 失败: %v", err)
	}
	fake := &fakeClient{repos: []gitapi.RemoteRepo{{Name: "cube", CloneUrl: "https://github.com/heyuuu/cube.git"}}}
	s.newClient = func(string, string, string) (gitapi.Client, error) { return fake, nil }

	repos, err := s.FetchNamespace("github.com", "heyuuu", false)
	if err != nil || len(repos) != 1 {
		t.Fatalf("首次拉取失败: repos=%+v err=%v", repos, err)
	}
	if _, err := s.FetchNamespace("github.com", "heyuuu", false); err != nil {
		t.Fatalf("二次拉取失败: %v", err)
	}
	if fake.lists != 1 {
		t.Fatalf("缓存命中不应重拉, lists=%d", fake.lists)
	}
	if _, err := s.FetchNamespace("github.com", "heyuuu", true); err != nil || fake.lists != 2 {
		t.Fatalf("force 应重拉: lists=%d err=%v", fake.lists, err)
	}

	// 对账：本地两条，其中一条属该 namespace 且匹配；另一条 host 不同不入范围
	local := []LocalRepo{
		{Name: "cube", Path: "~/src/cube", RepoUrl: "git@github.com:heyuuu/cube.git", Ahead: 1},
		{Name: "elsewhere", Path: "~/src/x", RepoUrl: "git@github.com:other/x.git"},
	}
	result, err := s.ReconcileNamespace("github.com", "heyuuu", local)
	if err != nil {
		t.Fatalf("对账失败: %v", err)
	}
	if len(result.Synced) != 1 || result.Synced[0].Local.Name != "cube" || len(result.Missing) != 0 || len(result.Orphan) != 0 {
		t.Fatalf("对账结果不符: %+v", result)
	}
}

// TestFetchPersist 拉取结果落盘：重启（新 Service 同配置）后缓存仍命中，不重拉。
func TestFetchPersist(t *testing.T) {
	s, ws := newService(t)
	seedForge(t, s)
	if err := s.SaveNamespace(Namespace{ForgeHost: "github.com", Path: "heyuuu", Type: "personal"}); err != nil {
		t.Fatalf("保存 namespace 失败: %v", err)
	}
	fake := &fakeClient{repos: []gitapi.RemoteRepo{{Name: "cube", CloneUrl: "https://github.com/heyuuu/cube.git"}}}
	s.newClient = func(string, string, string) (gitapi.Client, error) { return fake, nil }

	if _, err := s.FetchNamespace("github.com", "heyuuu", false); err != nil {
		t.Fatalf("拉取失败: %v", err)
	}
	if fake.lists != 1 {
		t.Fatalf("应拉取一次, got %d", fake.lists)
	}

	// 模拟重启：新 Service 指向同一落盘文件
	s2 := NewService(ws.Join("settings.json"), ws.Join("cache/forge-repos.json"))
	s2.newClient = s.newClient
	entry, ok := s2.CachedNamespaceRepos("github.com", "heyuuu")
	if !ok || len(entry.Repos) != 1 || entry.FetchedAt.IsZero() {
		t.Fatalf("重启后应从落盘恢复且带 fetchedAt: %+v ok=%v", entry, ok)
	}
	if _, err := s2.FetchNamespace("github.com", "heyuuu", false); err != nil {
		t.Fatalf("缓存命中拉取失败: %v", err)
	}
	if fake.lists != 1 {
		t.Fatalf("落盘命中不应重拉, lists=%d", fake.lists)
	}
}
