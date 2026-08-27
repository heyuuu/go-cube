package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"cube/internal/testfixture"
)

// writeCubeFile 在根下写 .cube/cube.json。
func writeCubeFile(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".cube")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cube.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// makeMonorepo 建一个含给定子目录的 monorepo 骨架（无 git，workspace 解析不需要）。
func makeMonorepo(t *testing.T, subdirs ...string) string {
	t.Helper()
	root := testfixture.NewWorkspace(t).Dir
	for _, sub := range subdirs {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestResolveExplicit(t *testing.T) {
	root := makeMonorepo(t, "server", "web")
	writeCubeFile(t, root, `{"workspaces":[{"name":"服务端","path":"server"},{"name":"web 前端","path":"web"}]}`)

	got := Resolve(root)
	want := []Workspace{{Name: "服务端", Path: "server"}, {Name: "web 前端", Path: "web"}}
	if len(got) != len(want) {
		t.Fatalf("Resolve() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Resolve()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// 空数组是显式声明「没有任何 workspace」，不回落探测（定稿语义）。
func TestResolveExplicitEmptyBeatsDetection(t *testing.T) {
	root := makeMonorepo(t, "apps/web")
	writeCubeFile(t, root, `{"workspaces":[],"workspaceScanRule":"pnpm"}`)
	writePnpmWorkspace(t, root, "apps/*")

	if got := Resolve(root); len(got) != 0 {
		t.Fatalf("空 workspaces 应等价于无成员，不回落探测，got %v", got)
	}
}

// 坏条目逐个跳过：绝对路径 / 逃逸 / 不存在；好条目保留。
func TestResolveSkipsBadEntries(t *testing.T) {
	root := makeMonorepo(t, "server")
	writeCubeFile(t, root, `{"workspaces":[
		{"name":"好条目","path":"server"},
		{"name":"绝对路径","path":"/etc"},
		{"name":"逃逸","path":"../outside"},
		{"name":"不存在","path":"gone"},
		{"name":"缺路径"}
	]}`)

	got := Resolve(root)
	if len(got) != 1 || got[0].Name != "好条目" || got[0].Path != "server" {
		t.Fatalf("坏条目应逐个跳过，got %v", got)
	}
}

// workspaces 字段不存在 → workspaceScanRule 生效。
func TestResolveScanRuleWhenFieldAbsent(t *testing.T) {
	root := makeMonorepo(t, "apps/web")
	writeCubeFile(t, root, `{"workspaceScanRule":"pnpm"}`)
	writePnpmWorkspace(t, root, "apps/*")

	got := Resolve(root)
	if len(got) != 1 || got[0].Path != "apps/web" {
		t.Fatalf("scanRule 探测应生效，got %v", got)
	}
}

// cube.json 不存在 → 默认规则探测；坏 JSON 等价于无声明。
func TestResolveDefaultAndBadJson(t *testing.T) {
	root := makeMonorepo(t, "apps/web")
	writePnpmWorkspace(t, root, "apps/*")

	got := Resolve(root)
	if len(got) != 1 || got[0].Path != "apps/web" {
		t.Fatalf("无 cube.json 应用默认规则探测，got %v", got)
	}

	writeCubeFile(t, root, `{bad json`)
	if got := Resolve(root); len(got) != 1 || got[0].Path != "apps/web" {
		t.Fatalf("坏 JSON 应等价无声明走探测，got %v", got)
	}
}

func writePnpmWorkspace(t *testing.T, root, pattern string) {
	t.Helper()
	content := "packages:\n  - '" + pattern + "'\n"
	if err := os.WriteFile(filepath.Join(root, "pnpm-workspace.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectPnpm(t *testing.T) {
	root := makeMonorepo(t, "apps/web", "apps/api", "packages/ui", "packages/ui/src", "docs")
	writePnpmWorkspace(t, root, "apps/*")

	got := Detect(root, "pnpm")
	// apps/* 只命中一层：apps/web、apps/api；packages 未声明不进结果
	if len(got) != 2 || got[0].Path != "apps/api" || got[1].Path != "apps/web" {
		t.Fatalf("Detect(pnpm) = %v", got)
	}
}

func TestDetectPnpmDoubleStar(t *testing.T) {
	root := makeMonorepo(t, "libs/a/core", "docs")
	writePnpmWorkspace(t, root, "libs/**")

	got := Detect(root, "pnpm")
	if len(got) != 2 { // libs/a 与 libs/a/core（** 递归）
		t.Fatalf("Detect(pnpm **) = %v", got)
	}
}

func TestDetectNpmListAndObject(t *testing.T) {
	// 数组形态
	root := makeMonorepo(t, "apps/web", "packages/ui")
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"m","workspaces":["apps/*","packages/*"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Detect(root, "npm"); len(got) != 2 {
		t.Fatalf("Detect(npm list) = %v", got)
	}

	// 对象形态（nohoist 等场景）
	root2 := makeMonorepo(t, "apps/web")
	if err := os.WriteFile(filepath.Join(root2, "package.json"), []byte(`{"workspaces":{"packages":["apps/*"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Detect(root2, "npm"); len(got) != 1 || got[0].Path != "apps/web" {
		t.Fatalf("Detect(npm object) = %v", got)
	}
}

// 规则按序生效：第一个命中的探测器赢，后续不再看。
func TestDetectOrder(t *testing.T) {
	root := makeMonorepo(t, "apps/web")
	writePnpmWorkspace(t, root, "apps/*")
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"workspaces":["apps/*"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// pnpm 命名来源优先：两处都展开 apps/web，名字应同为 web——用来源区分不可行，
	// 这里验证的是「pnpm,npm 与 npm,pnpm 结果一致（都命中同一目录集）」以及未知规则跳过
	for _, rule := range []string{"pnpm,npm", "npm,pnpm", "ghost,pnpm"} {
		if got := Detect(root, rule); len(got) != 1 {
			t.Fatalf("Detect(%s) = %v, want 1 个成员", rule, got)
		}
	}
	if got := Detect(root, "ghost"); got != nil {
		t.Fatalf("全部未知规则应无结果，got %v", got)
	}
}

// glob 排除项：node_modules / 点前缀目录不进结果。
func TestDetectExcludes(t *testing.T) {
	root := makeMonorepo(t, "apps/web", "apps/node_modules/pkg", "apps/.hidden")
	writePnpmWorkspace(t, root, "apps/*")

	got := Detect(root, "pnpm")
	if len(got) != 1 || got[0].Path != "apps/web" {
		t.Fatalf("应排除 node_modules 与隐藏目录，got %v", got)
	}
}
