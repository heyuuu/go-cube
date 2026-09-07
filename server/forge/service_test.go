package forge

import (
	"context"
	"testing"

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
	if err := s.SaveForge(Forge{Host: "GitHub.COM", Kind: KindGithub}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if err := s.SaveForge(Forge{Host: "github.com", Kind: "unknown"}); err == nil {
		t.Fatal("未知 kind 应报错")
	}
	if len(s.Forges()) != 1 {
		t.Fatalf("同键替换后应只剩一条: %+v", s.Forges())
	}
}

// --- account（1041/1044） ---

// seedForge 写入一个 github.com forge，供 account 校验命中。
func seedForge(t *testing.T, s *Service) {
	t.Helper()
	if err := s.SaveForge(Forge{Host: "github.com", Kind: KindGithub}); err != nil {
		t.Fatalf("seed forge 失败: %v", err)
	}
}

// TestAccountCrud account 保存/替换/删除 + 校验（forge 未配置 / generic 拒配）+ 掩码语义。
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

// TestDeleteForgeCascade 删 forge 级联清理该 host 下的 account。
func TestDeleteForgeCascade(t *testing.T) {
	s, _ := newService(t)
	seedForge(t, s)
	if err := s.SaveForge(Forge{Host: "gitea.example.com", Kind: KindGitea}); err != nil {
		t.Fatalf("seed 失败: %v", err)
	}
	for _, a := range []Account{
		{ForgeHost: "github.com", Username: "heyuuu", Token: "t"},
		{ForgeHost: "gitea.example.com", Username: "ops", Token: "t"},
	} {
		if err := s.SaveAccount(a); err != nil {
			t.Fatalf("保存 account 失败: %v", err)
		}
	}

	if err := s.DeleteForge("github.com"); err != nil {
		t.Fatalf("删 forge 失败: %v", err)
	}
	if len(s.Accounts()) != 1 || s.Accounts()[0].ForgeHost != "gitea.example.com" {
		t.Fatalf("account 应只剩 gitea 一条: %+v", s.Accounts())
	}
}

// fakeClient 固定返回预置结果（List 调用计数供断言）。
type fakeClient struct {
	repos []gitapi.RemoteRepo
	lists int
}

func (f *fakeClient) ListAccountRepos(_ context.Context) ([]gitapi.RemoteRepo, error) {
	f.lists++
	return f.repos, nil
}

// TestFetchAccountPersist 拉取缓存（命中不重拉）+ 落盘（新 Service 同配置恢复不重拉）。
func TestFetchAccountPersist(t *testing.T) {
	s, ws := newService(t)
	seedForge(t, s)
	if err := s.SaveAccount(Account{ForgeHost: "github.com", Username: "heyuuu", Token: "t"}); err != nil {
		t.Fatalf("保存 account 失败: %v", err)
	}
	fake := &fakeClient{repos: []gitapi.RemoteRepo{{Name: "cube", CloneUrl: "https://github.com/heyuuu/cube.git"}}}
	s.newClient = func(string, string, string) (gitapi.Client, error) { return fake, nil }

	repos, err := s.FetchAccount("github.com", "heyuuu", false)
	if err != nil || len(repos) != 1 {
		t.Fatalf("首次拉取失败: repos=%+v err=%v", repos, err)
	}
	if _, err := s.FetchAccount("github.com", "heyuuu", false); err != nil {
		t.Fatalf("二次拉取失败: %v", err)
	}
	if fake.lists != 1 {
		t.Fatalf("缓存命中不应重拉, lists=%d", fake.lists)
	}

	// 模拟重启：新 Service 指向同一落盘文件
	s2 := NewService(ws.Join("settings.json"), ws.Join("cache/forge-repos.json"))
	s2.newClient = s.newClient
	entry, ok := s2.CachedAccountRepos("github.com", "heyuuu")
	if !ok || len(entry.Repos) != 1 || entry.FetchedAt.IsZero() {
		t.Fatalf("重启后应从落盘恢复且带 fetchedAt: %+v ok=%v", entry, ok)
	}
	if _, err := s2.FetchAccount("github.com", "heyuuu", false); err != nil || fake.lists != 1 {
		t.Fatalf("落盘命中不应重拉: lists=%d err=%v", fake.lists, err)
	}
	if _, err := s.FetchAccount("github.com", "heyuuu", true); err != nil || fake.lists != 2 {
		t.Fatalf("force 应重拉: lists=%d err=%v", fake.lists, err)
	}
}

// settings 引用占位（iconkit/settings 在裁剪后的用例中仍被间接使用，防止 import 漂移）。
var (
	_ = settings.SaveSection
	_ = iconkit.ValidateIcon
)
