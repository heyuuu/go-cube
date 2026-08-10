package pathkit

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------- RealPath ----------

func TestRealPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() 失败: %v", err)
	}

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"tilde 展开", "~/code", filepath.Join(home, "code")},
		{"无 tilde 原样返回", "/usr/local/bin", "/usr/local/bin"},
		{"相对路径原样返回", "foo/bar", "foo/bar"},
		{"仅波浪号本身不展开(非 ~/ 前缀)", "~user/x", "~user/x"},
		{"空串原样返回", "", ""},
		{"单个 ~ 指代 home 目录", "~", home},

		// 脏输入：冗余分隔符 / . / .. 按 filepath.Clean 规范化
		{"绝对路径双斜杠折叠", "/usr//local/bin", "/usr/local/bin"},
		{"绝对路径 . 段清理", "/usr/./local/bin", "/usr/local/bin"},
		{"绝对路径 .. 回退", "/usr/local/../bin", "/usr/bin"},
		{"tilde 展开含双斜杠", "~/code//app", filepath.Join(home, "code", "app")},
		{"tilde 展开含 . 段", "~/code/./app", filepath.Join(home, "code", "app")},
		{"相对路径双斜杠折叠", "foo//bar", "foo/bar"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RealPath(c.in); got != c.want {
				t.Fatalf("RealPath(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// ---------- ResolvePath ----------

func TestResolvePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() 失败: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() 失败: %v", err)
	}

	cases := []struct {
		name string
		in   string
		want string
	}{
		// 已是绝对路径：原样返回（Clean 规范化冗余段）
		{"绝对路径原样返回", "/usr/local/bin", "/usr/local/bin"},
		{"绝对路径脏输入双斜杠", "/usr//local/bin", "/usr/local/bin"},
		{"绝对路径脏输入点段", "/usr/./local/bin", "/usr/local/bin"},
		{"绝对路径脏输入双点回退", "/usr/local/../bin", "/usr/bin"},

		// ~/ 前缀：先展开成 home 绝对路径（不再相对 cwd）
		{"tilde 展开为绝对", "~/code", filepath.Join(home, "code")},
		{"tilde 展开含脏输入", "~/code//app", filepath.Join(home, "code", "app")},
		// 仅波浪号本身不展开（非 ~/ 前缀），按相对路径处理 → 相对 cwd
		{"仅波浪号走相对路径", "~user/x", filepath.Join(wd, "~user", "x")},

		// 相对路径：基于 cwd 转 abs
		{"相对路径转绝对", "foo/bar", filepath.Join(wd, "foo", "bar")},
		{"相对路径脏输入双斜杠", "foo//bar", filepath.Join(wd, "foo", "bar")},
		{"相对路径点段", "foo/./bar", filepath.Join(wd, "foo", "bar")},
		{"相对路径双点回退", "a/b/../../c", filepath.Join(wd, "c")},
		{"相对单点", "./app", filepath.Join(wd, "app")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ResolvePath(c.in)
			if err != nil {
				t.Fatalf("ResolvePath(%q) 出错: %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("ResolvePath(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// ---------- PrettyPath ----------

func TestPrettyPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() 失败: %v", err)
	}

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"home 目录下转 ~", filepath.Join(home, "code", "app"), "~/code/app"},
		{"home 目录本身转 ~", home, "~"},
		{"home 目录带斜杠转 ~", home + "/", "~"},
		{"home 目录外保持绝对", "/usr/local/bin", "/usr/local/bin"},
		{"相对路径原样返回", "foo/bar", "foo/bar"},
		{"空串原样返回", "", ""},

		// 脏输入：冗余分隔符 / . / .. 按 filepath.Clean 规范化后再转 ~
		{"home 下双斜杠折叠", filepath.Join(home, "code") + "//app", "~/code/app"},
		{"home 下 . 段清理", filepath.Join(home, "code", ".", "app"), "~/code/app"},
		{"home 下 .. 回退", filepath.Join(home, "code", "sub", "..", "app"), "~/code/app"},
		{"home 下 .. 回退", filepath.Join(home, "..", "code"), filepath.Clean(filepath.Join(home, "..", "code"))},
		{"home 外双斜杠折叠", "/usr//local/bin", "/usr/local/bin"},
		{"home 外 . 段清理", "/usr/./local/bin", "/usr/local/bin"},
		{"home 外 .. 回退", "/usr/local/../bin", "/usr/bin"},
		{"相对路径 .. 回退-1", "a/./b", "a/b"},
		{"相对路径 .. 回退-2", "a/../../b", "../b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PrettyPath(c.in); got != c.want {
				t.Fatalf("PrettyPath(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// ---------- CommonPrefix ----------

func TestCommonPrefix(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  string
	}{
		// 边界
		{"空切片返回空", []string{}, ""},
		// 单条返回 Clean 后的自身（不取父目录——本函数只做前缀运算）
		{"单条返回自身", []string{"/a/b/c"}, "/a/b/c"},
		{"单条相对路径返回自身", []string{"a/b"}, "a/b"},
		{"单条脏输入Clean", []string{"/a//b/./c"}, "/a/b/c"},

		// 多路径公共前缀
		{"兄弟项目公共前缀", []string{"/a/b/proj1", "/a/b/proj2"}, "/a/b"},
		{"不同分组取更高层", []string{"/a/x/p1", "/a/y/p2"}, "/a"},
		// 多条相同路径：公共前缀 = 路径本身（与单条一致）
		{"相同路径取自身", []string{"/a/b/c", "/a/b/c"}, "/a/b/c"},
		// a 是 b 的前缀（两种顺序都应得到相同结果）
		{"前缀包含关系-短在前", []string{"/a/b", "/a/b/c"}, "/a/b"},
		{"前缀包含关系-长在前", []string{"/a/b/c", "/a/b"}, "/a/b"},

		// 前缀陷阱：按段比较，不应把 "a/ab" 误判为 "a/ab" 的前缀
		{"前缀段级匹配 ab vs abc", []string{"/a/ab", "/a/abc"}, "/a"},

		// 无公共前缀：绝对回退到根 "/"，相对返回 ""
		{"完全无公共前缀回退根", []string{"/a/b", "/x/y"}, "/"},
		{"多条绝对路径无公共前缀回退根", []string{"/a/b", "/x/y", "/m/n"}, "/"},
		{"多条相对路径无公共前缀返回空", []string{"a/b", "x/y"}, ""},

		// 绝对/相对混杂：首字符不同即无公共，返回 ""
		{"绝对相对混杂返回空", []string{"/a/b", "c/d"}, ""},
		{"多条混杂返回空", []string{"/a/b", "c/d", "/e/f"}, ""},

		// 脏输入：冗余分隔符 / . / .. 先经 filepath.Clean 再求公共
		{"绝对路径脏输入双斜杠", []string{"/a//b/proj1", "/a/b/proj2"}, "/a/b"},
		{"绝对路径脏输入点段", []string{"/a/./b/proj1", "/a/b/proj2"}, "/a/b"},
		{"绝对路径脏输入双点回退", []string{"/a/b/../b/proj1", "/a/b/proj2"}, "/a/b"},
		{"绝对路径脏输入混合", []string{"/a//b", "/a/./b/c", "/a/../a/b/d"}, "/a/b"},
		{"相对路径脏输入双斜杠", []string{"a//b/proj1", "a/b/proj2"}, "a/b"},
		{"相对路径脏输入双点回退", []string{"a/b/sub/../proj1", "a/b/proj2"}, "a/b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CommonPrefix(c.paths); got != c.want {
				t.Fatalf("CommonPrefix(%v) = %q, want %q", c.paths, got, c.want)
			}
		})
	}
}
