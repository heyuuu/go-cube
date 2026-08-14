package pathkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- resolveAbs / StaticAbsPath / AbsPath ----------

// TestResolveAbs 绝对化核心逻辑：baseDir 注入式表驱动，不依赖进程 cwd。
func TestResolveAbs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // 固定 home，让 ~ 展开结果可预期

	cases := []struct {
		name    string
		in      string
		baseDir string
		want    string
		wantErr bool
	}{
		// 绝对路径：baseDir 不参与（Node path.resolve 的「绝对参数覆盖基准」语义）
		{"绝对路径无视 baseDir", "/abs/path", "/base", "/abs/path", false},
		{"绝对路径无 baseDir", "/abs/path", "", "/abs/path", false},
		{"绝对路径脏输入归一化", "/abs//x/./y/", "", "/abs/x/y", false},
		{"绝对路径含 .. 回退", "/abs/sub/../x", "", "/abs/x", false},

		// 空串
		{"空串报错", "", "", "", true},
		{"空串带 baseDir 也报错", "", "/base", "", true},

		// ~ 前缀展开（优先于 baseDir）
		{"~ 指 home", "~", "", home, false},
		{"~/ 指 home", "~/", "", home, false},
		{"~/sub/x 展开", "~/sub/x", "", filepath.Join(home, "sub", "x"), false},
		{"~ 展开脏输入归一化", "~/a//b/./c", "", filepath.Join(home, "a", "b", "c"), false},
		{"~ 优先于 baseDir", "~/x", "/base", filepath.Join(home, "x"), false},

		// 相对路径：仅接受 . / .. / ./x / ../x 显式语法（与 cmd 层 isPathQuery 的前缀分流对齐）
		{"单点指 baseDir", ".", "/base", "/base", false},
		{"./x 基于 baseDir", "./x", "/base", "/base/x", false},
		{"../x 越过 baseDir 下层", "../x", "/base/sub", "/base/x", false},
		{".. 指 baseDir 父", "..", "/base/sub", "/base", false},
		{"相对脏输入归一化", "./a//b/./c", "/base", "/base/a/b/c", false},
		{"相对 .. 回退出 baseDir", "../../x", "/base/sub", "/x", false},
		{"baseDir 带尾斜杠", "./x", "/base/", "/base/x", false},
		{"不是相对路径但以 . 开头的写法", ".abc", "/base/", "", true},
		{"隐藏名以 . 开头是裸相对", ".git", "/base", "", true},

		// 裸相对路径：不收（cmd 层走 name 分支，到这里说明调用方传错）
		{"裸相对路径报错", "sub/x", "/base", "", true},
		{"裸相对无基准报错", "x", "", "", true},

		// ~user 语法不支持：非 ~ / ~/ 前缀，落到报错
		{"~user 语法报错", "~user/x", "/base", "", true},

		// 显式相对但无 baseDir：报错（StaticAbsPath 语义）
		{"相对路径无基准报错", "./x", "", "", true},
		{".. 无基准报错", "..", "", "", true},

		// baseDir 不变量断言
		{"baseDir 相对路径报错", "x", "relative-base", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveAbs(c.in, c.baseDir)
			if c.wantErr {
				if err == nil {
					t.Fatalf("resolveAbs(%q, %q) 期望报错，实际返回 %q", c.in, c.baseDir, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveAbs(%q, %q) 出错: %v", c.in, c.baseDir, err)
			}
			if got != c.want {
				t.Fatalf("resolveAbs(%q, %q) = %q, want %q", c.in, c.baseDir, got, c.want)
			}
		})
	}
}

// TestResolveAbs_HomeUnavailable HOME 缺失时 ~ 展开报错，而非静默降级。
func TestResolveAbs_HomeUnavailable(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := resolveAbs("~/x", ""); err == nil {
		t.Fatal("HOME 缺失时 ~/ 展开应报错")
	}
}

// TestResolveAbs_InvariantFirst p 与 baseDir 双非法时，不变量断言（代码 bug）优先于判空（业务错误）。
func TestResolveAbs_InvariantFirst(t *testing.T) {
	_, err := resolveAbs("", "relative-base")
	if err == nil || !strings.Contains(err.Error(), "baseDir") {
		t.Fatalf("双非法时应优先报 baseDir 不变量错误，实际: %v", err)
	}
}

// TestStaticAbsPath 静态绝对化糖层：仅接受绝对路径与 ~ 前缀，相对路径一律报错。
func TestStaticAbsPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	okCases := []struct{ name, in, want string }{
		{"绝对路径", "/abs/path", "/abs/path"},
		{"~ 展开", "~/code", filepath.Join(home, "code")},
		{"~ 本身指 home", "~", home},
	}
	for _, c := range okCases {
		t.Run(c.name, func(t *testing.T) {
			got, err := StaticAbsPath(c.in)
			if err != nil {
				t.Fatalf("StaticAbsPath(%q) 出错: %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("StaticAbsPath(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}

	errCases := []struct{ name, in string }{
		{"./ 相对路径", "./x"},
		{"裸相对路径", "x"},
		{"上跳相对路径", "../x"},
		{"空串", ""},
	}
	for _, c := range errCases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := StaticAbsPath(c.in); err == nil {
				t.Fatalf("StaticAbsPath(%q) 相对路径应报错", c.in)
			}
		})
	}
}

// TestAbsPath 基于 cwd 的绝对化糖层：相对路径以进程当前目录为基准。
func TestAbsPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	t.Chdir(dir) // 固定 cwd，测试不依赖外部目录

	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"单点指 cwd", ".", dir, false},
		{"./sub 基于 cwd", "./sub", filepath.Join(dir, "sub"), false},
		{"../x 越过 cwd", "../x", filepath.Clean(filepath.Join(dir, "..", "x")), false},
		{"绝对路径覆盖 cwd", "/abs", "/abs", false},
		{"~ 展开优先于 cwd", "~/code", filepath.Join(home, "code"), false},
		{"空串报错", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := AbsPath(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("AbsPath(%q) 期望报错，实际返回 %q", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("AbsPath(%q) 出错: %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("AbsPath(%q) = %q, want %q", c.in, got, c.want)
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
