package opener

import (
	"strings"
	"testing"
)

func TestParseIntent(t *testing.T) {
	for _, it := range intentOrder {
		got, err := ParseIntent(string(it))
		if err != nil || got != it {
			t.Fatalf("ParseIntent(%q) = (%q,%v)", it, got, err)
		}
	}
	if _, err := ParseIntent("nope"); err == nil || !strings.Contains(err.Error(), "未知的 opener intent") {
		t.Fatalf("未知 intent 应报中文错误, got %v", err)
	}
}

// intent → role 唯一映射；所有枚举值都有映射
func TestIntentRoleMapping(t *testing.T) {
	cases := map[Intent]Role{
		IntentDir: RoleOpenDir, IntentFile: RoleOpenFile,
		IntentDiffDir: RoleDiffDir, IntentDiffFile: RoleDiffFile,
		IntentTerminal: RoleOpenDir, IntentGit: RoleOpenDir,
		IntentWorkbench: RoleOpenDir, IntentIde: RoleOpenDir, IntentDoc: RoleOpenFile,
	}
	if len(cases) != len(intentOrder) {
		t.Fatalf("映射表与枚举数量不一致: %d vs %d", len(cases), len(intentOrder))
	}
	for it, want := range cases {
		if it.Role() != want {
			t.Fatalf("%s.Role() = %s, want %s", it, it.Role(), want)
		}
	}
}
