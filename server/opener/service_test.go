package opener

import (
	"testing"

	"cube/internal/testfixture"
	"cube/settings"
)

// newServiceAt 建一个指向测试专属 settings.json 的 Service（注入 fakeExecutor）。
func newServiceAt(t *testing.T, specs []Spec) (*Service, *fakeExecutor) {
	t.Helper()
	ws := testfixture.NewWorkspace(t)
	file := ws.Join("settings.json")
	if specs != nil {
		if err := settings.SaveSection(file, settingsSection, specs); err != nil {
			t.Fatalf("写入测试 settings.json 失败: %v", err)
		}
	}
	fake := &fakeExecutor{}
	return NewService(file, fake), fake
}

func TestServiceDirectRead(t *testing.T) {
	t.Run("文件不存在返回空列表", func(t *testing.T) {
		s, _ := newServiceAt(t, nil)
		if got := s.AllOpeners(); len(got) != 0 {
			t.Fatalf("期望空列表, got %v", got)
		}
	})

	t.Run("读取全部与按名查找", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{
			{Name: "code", Cmd: []string{"code"}},
			{Name: "bcompare", Cmd: []string{"bcompare", "$0", "$1"}, Roles: []string{"diff-dir"}},
		})
		if got := s.AllOpeners(); len(got) != 2 {
			t.Fatalf("期望 2 个, got %d", len(got))
		}
		if o := s.FindByName("bcompare"); o == nil {
			t.Fatal("FindByName(bcompare) 应命中")
		}
		if o := s.FindByName("nope"); o != nil {
			t.Fatalf("FindByName(nope) 不应命中, got %v", o)
		}
	})

	t.Run("坏条目跳过不阻断", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{
			{Name: "bad", Cmd: nil},                  // 缺 cmd，条目级坏
			{Name: "code", Cmd: []string{"code"}},    // 正常
			{Name: "bad2", Cmd: []string{"c", "$9"}}, // 占位符越界，条目级坏
		})
		got := s.AllOpeners()
		if len(got) != 1 || got[0].Name() != "code" {
			t.Fatalf("期望只剩 code, got %v", got)
		}
	})

	t.Run("RoleOpeners 过滤", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{
			{Name: "code", Cmd: []string{"code"}},
			{Name: "bcompare", Cmd: []string{"bcompare", "$0", "$1"}, Roles: []string{"diff-dir"}},
		})
		if got := s.RoleOpeners(RoleDiffDir); len(got) != 1 || got[0].Name() != "bcompare" {
			t.Fatalf("diff-dir 应只有 bcompare, got %v", got)
		}
	})

	t.Run("SearchAll 模糊匹配", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{
			{Name: "code", Cmd: []string{"code"}},
			{Name: "vscode", Cmd: []string{"vscode"}},
		})
		if got := s.SearchAll("cod"); len(got) != 2 {
			t.Fatalf("cod 应匹配 code+vscode, got %v", got)
		}
	})

	t.Run("保存后无需重建 Service 即生效（直读不缓存）", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{{Name: "code", Cmd: []string{"code"}}})
		if err := settings.SaveSection(s.settingsFile, settingsSection,
			[]Spec{{Name: "newone", Cmd: []string{"newone"}}}); err != nil {
			t.Fatalf("写入失败: %v", err)
		}
		if o := s.FindByName("newone"); o == nil {
			t.Fatal("新条目应立即可见（直读）")
		}
		if o := s.FindByName("code"); o != nil {
			t.Fatalf("旧条目应已消失, got %v", o)
		}
	})

	t.Run("其他节不受影响", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{{Name: "code", Cmd: []string{"code"}}})
		if err := settings.SaveSection(s.settingsFile, "other", map[string]int{"a": 1}); err != nil {
			t.Fatalf("写入失败: %v", err)
		}
		if got := s.AllOpeners(); len(got) != 1 || got[0].Name() != "code" {
			t.Fatalf("写其他节不应影响 openers, got %v", got)
		}
	})
}
