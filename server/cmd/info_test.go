package cmd

import (
	"strings"
	"testing"

	"cube/util/gogit"
)

// stripAnsi 去掉 lipgloss 渲染产生的 ANSI 转义码，便于对输出做内容断言。
func stripAnsi(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEsc = true
		case inEsc && r == 'm':
			inEsc = false
		case !inEsc:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TestBuildSyncSuggestions 验证按差距生成 pull/push 建议：
// 仅落后给 pull、仅领先给 push、分叉只提示不给命令、已同步不产生条目；
// 输出顺序为 pull 在前 push 在后（先更新本地再推送）。
func TestBuildSyncSuggestions(t *testing.T) {
	diffs := []branchRemoteDiff{
		{Branch: "develop", Remote: "origin", Ahead: 1, Behind: 0},
		{Branch: "master", Remote: "origin", Ahead: 0, Behind: 2},
		{Branch: "feature", Remote: "gitee", Ahead: 2, Behind: 3},
		{Branch: "synced", Remote: "gitee", Ahead: 0, Behind: 0},
	}

	got := buildSyncSuggestions(diffs, "/repo")
	if len(got) != 3 {
		t.Fatalf("应产生 3 条建议（已同步不产生），实际 %d: %+v", len(got), got)
	}

	// 落后 → pull 命令 + 远端领先注释（排在最前）
	if got[0].Command != "cube pull /repo -r origin -b master" || got[0].Note != "远端领先 2" {
		t.Errorf("落后分支建议 = %+v", got[0])
	}
	// 领先 → push 命令 + 本地领先注释
	if got[1].Command != "cube push /repo -r origin -b develop" || got[1].Note != "本地领先 1" {
		t.Errorf("领先分支建议 = %+v", got[1])
	}
	// 分叉 → 无命令，仅提示（排最后）
	if got[2].Command != "" || !strings.Contains(got[2].Note, "feature 与 gitee 分叉") {
		t.Errorf("分叉分支建议应为仅提示，实际 %+v", got[2])
	}
}

// TestBuildInfoRemoteLines 多 remote 行：名字列按最长名对齐，行内含地址与网页地址。
func TestBuildInfoRemoteLines(t *testing.T) {
	lines := buildInfoRemoteLines([]gogit.Remote{
		{Name: "origin", Fetch: "git@github.com:heyuuu/cube.git"},
		{Name: "gitee", Fetch: "git@gitee.com:heyuuu/cube.git"},
	})

	if len(lines) != 2 {
		t.Fatalf("应 2 行，实际 %d: %v", len(lines), lines)
	}
	for i, want := range []struct{ name, url, web string }{
		{"origin", "git@github.com:heyuuu/cube.git", "(https://github.com/heyuuu/cube)"},
		{"gitee", "git@gitee.com:heyuuu/cube.git", "(https://gitee.com/heyuuu/cube)"},
	} {
		plain := stripAnsi(lines[i])
		if !strings.HasPrefix(plain, want.name) || !strings.Contains(plain, want.url) || !strings.Contains(plain, want.web) {
			t.Errorf("行 %d = %q，期望包含 %s / %s / %s", i, plain, want.name, want.url, want.web)
		}
	}
	// 名字列对齐：两行 url 起始列一致（剥 ANSI 后）
	a, b := stripAnsi(lines[0]), stripAnsi(lines[1])
	if strings.Index(a, "git@") != strings.Index(b, "git@") {
		t.Errorf("名字列未对齐:\n%q\n%q", a, b)
	}
}
