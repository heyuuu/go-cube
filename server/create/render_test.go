package create

import (
	"os"
	"path/filepath"
	"testing"

	"cube/internal/testfixture"
)

// makeTemplateDir 建一个含占位符目录名、多层文件、二进制文件的模板目录。
func makeTemplateDir(t *testing.T, ws *testfixture.Workspace, name string) string {
	dir := ws.Mkdir(name)
	ws.WriteFile(filepath.Join(name, "template.yaml"), []byte("version: 1\n"))
	ws.WriteFile(filepath.Join(name, "__PROJECT__/main.go"), []byte("package main // __MODULE__"))
	ws.WriteFile(filepath.Join(name, "__PROJECT__/nested/util.go"), []byte("// __PROJECT__"))
	ws.WriteFile(filepath.Join(name, "README.md"), []byte("# __PROJECT__"))
	if err := os.WriteFile(ws.Join(name, "logo.png"), []byte("PN\x00G"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func testTemplateYaml(t *testing.T) *TemplateYaml {
	tpl, err := InitTemplateYaml([]byte(`
version: 1
variables:
  project-name: {prompt: 项目名, required: true}
  module: {prompt: module, required: true}
patterns:
  "**/*.go":
    - {pattern: __MODULE__, replace: "github.com/x/${module}"}
    - {pattern: __PROJECT__, replace: "${project-name}"}
  "**/*.md":
    - {pattern: __PROJECT__, replace: "${project-name}"}
`))
	if err != nil {
		t.Fatal(err)
	}
	tpl.Patterns["**/*.go"][0].Replace = "github.com/x/myapp"
	tpl.Patterns["**/*.go"][1].Replace = "demo"
	tpl.Patterns["**/*.md"][0].Replace = "demo"
	return tpl
}

func TestRender(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	templateDir := makeTemplateDir(t, ws, "tpl")
	target := ws.Join("out")

	count, err := Render(templateDir, target, testTemplateYaml(t))
	if err != nil {
		t.Fatalf("Render 报错: %v", err)
	}
	// 4 个文件（main.go / nested/util.go / README.md / logo.png），template.yaml 不算
	if count != 4 {
		t.Fatalf("生成文件数 = %d, want 4", count)
	}

	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(target, rel))
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", rel, err)
		}
		return string(data)
	}

	// 目录名占位符替换（路径含文件名）
	if got := read("demo/main.go"); got != "package main // github.com/x/myapp" {
		t.Fatalf("main.go 内容替换失败: %q", got)
	}
	if got := read("demo/nested/util.go"); got != "// demo" {
		t.Fatalf("nested/util.go 替换失败: %q", got)
	}
	if got := read("README.md"); got != "# demo" {
		t.Fatalf("README 替换失败: %q", got)
	}
	// 二进制内容原样
	if got := read("logo.png"); got != "PN\x00G" {
		t.Fatalf("二进制文件内容被改动: %q", got)
	}
	// template.yaml 不被复制
	if _, err := os.Stat(filepath.Join(target, "template.yaml")); !os.IsNotExist(err) {
		t.Fatal("template.yaml 不应被复制到目标")
	}
}

func TestRenderSkipDotGit(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	templateDir := makeTemplateDir(t, ws, "tpl")
	ws.WriteFile(filepath.Join("tpl", ".git/config"), []byte("[core]"))
	ws.WriteFile(filepath.Join("tpl", ".git/objects/x"), []byte("x"))

	target := ws.Join("out")
	count, err := Render(templateDir, target, testTemplateYaml(t))
	if err != nil {
		t.Fatalf("Render 报错: %v", err)
	}
	if count != 4 {
		t.Fatalf("生成文件数 = %d, want 4（.git 应被跳过）", count)
	}
	if _, err := os.Stat(filepath.Join(target, ".git")); !os.IsNotExist(err) {
		t.Fatal(".git 不应被生成")
	}
}

func TestRenderGlobEdges(t *testing.T) {
	// glob-rules.md 的边界行为：** 零层、dotfile 默认匹配、全路径匹配非子串
	tests := []struct {
		glob string
		rel  string
		want bool
	}{
		{"src/**/*.go", "src/a.go", true},      // ** 零层也算
		{"src/**/*.go", "src/x/a.go", true},    // 跨层
		{"src/*.go", "src/nested/a.go", false}, // * 不跨 /
		{"**/*.go", ".hidden.go", true},        // dotfile 默认匹配
		{"**/*.go", "src/b.txt", false},
		{"src/*.go", "xsrc/a.go", false}, // 全路径匹配，非子串
	}
	for _, tt := range tests {
		groups := []globGroup{{glob: tt.glob, rules: []ReplaceRule{{Pattern: "x"}}}}
		if got := len(collectRules(groups, tt.rel)) == 1; got != tt.want {
			t.Errorf("collectRules(%q, %q) 命中 = %v, want %v", tt.glob, tt.rel, got, tt.want)
		}
	}
}

func TestCompileGroupsErrors(t *testing.T) {
	tests := []struct {
		name string
		tpl  *TemplateYaml
	}{
		{"非法 glob", &TemplateYaml{Patterns: map[string][]ReplaceRule{"[": nil}}},
		{"空 pattern", &TemplateYaml{Patterns: map[string][]ReplaceRule{"**/*.go": {{Pattern: "", Replace: "x"}}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := compileGroups(tt.tpl); err == nil {
				t.Fatal("期望报错")
			}
		})
	}
}
