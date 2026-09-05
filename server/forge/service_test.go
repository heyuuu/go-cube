package forge

import (
	"strings"
	"testing"

	"cube/internal/testfixture"
	"cube/settings"
	"cube/util/iconkit"
)

func newService(t *testing.T) (*Service, *testfixture.Workspace) {
	t.Helper()
	ws := testfixture.NewWorkspace(t)
	return NewService(ws.Join("settings.json")), ws
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
