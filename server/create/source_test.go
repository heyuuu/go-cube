package create

import (
	"cube/config"

	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cube/internal/testfixture"
)

func TestInspectSource(t *testing.T) {
	ws := testfixture.NewWorkspace(t)

	t.Run("单模板", func(t *testing.T) {
		dir := ws.Mkdir("single")
		ws.WriteFile(filepath.Join("single", "template.yaml"), []byte("version: 1\n"))
		layout, err := InspectSource(dir)
		if err != nil {
			t.Fatalf("报错: %v", err)
		}
		if len(layout.TemplateNames) != 0 {
			t.Fatalf("应判定为单模板: %v", layout.TemplateNames)
		}
	})

	t.Run("模板集", func(t *testing.T) {
		dir := ws.Mkdir("set")
		for _, n := range []string{"full", "minimal", "with-docker"} {
			ws.WriteFile(filepath.Join("set", "templates", n, "template.yaml"), []byte("version: 1\n"))
		}
		// 干扰项：templates/ 内的非模板子目录、隐藏目录不算
		ws.Mkdir("set/templates/docs")
		ws.Mkdir("set/templates/.hidden")
		ws.WriteFile(filepath.Join("set/templates/docs/readme.md"), []byte("非模板目录"))
		// 根目录其他内容不参与判定：README、docs、草稿目录（即使含 template.yaml）均不可见
		ws.WriteFile(filepath.Join("set", "README.md"), []byte("库说明"))
		ws.WriteFile(filepath.Join("set", "drafts", "wip", "template.yaml"), []byte("version: 1\n"))
		layout, err := InspectSource(dir)
		if err != nil {
			t.Fatalf("报错: %v", err)
		}
		want := "full,minimal,with-docker" // 字典序
		if got := strings.Join(layout.TemplateNames, ","); got != want {
			t.Fatalf("模板集 = %q, want %q", got, want)
		}
	})

	t.Run("单模板优先于模板集", func(t *testing.T) {
		dir := ws.Mkdir("mixed")
		ws.WriteFile(filepath.Join("mixed", "template.yaml"), []byte("version: 1\n"))
		ws.WriteFile(filepath.Join("mixed", "templates/sub/template.yaml"), []byte("version: 1\n"))
		layout, err := InspectSource(dir)
		if err != nil {
			t.Fatalf("报错: %v", err)
		}
		if len(layout.TemplateNames) != 0 {
			t.Fatal("根目录有 template.yaml 时应判定为单模板")
		}
	})

	t.Run("两者都不是", func(t *testing.T) {
		dir := ws.Mkdir("empty")
		if _, err := InspectSource(dir); err == nil {
			t.Fatal("期望报错")
		}
	})

	t.Run("templates 为空报错", func(t *testing.T) {
		dir := ws.Mkdir("blankset")
		ws.Mkdir("blankset/templates")
		if _, err := InspectSource(dir); err == nil {
			t.Fatal("期望报错")
		}
	})
}

func TestSelectTemplateDir(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.Mkdir("set")
	ws.WriteFile(filepath.Join("set/templates/full/template.yaml"), []byte("version: 1\n"))
	ws.WriteFile(filepath.Join("set/templates/minimal/template.yaml"), []byte("version: 1\n"))
	layout, err := InspectSource(dir)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("指定名命中", func(t *testing.T) {
		got, err := SelectTemplateDir(layout, "full")
		if err != nil || filepath.Base(got) != "full" {
			t.Fatalf("got %q, err %v", got, err)
		}
	})

	t.Run("指定名不存在列出可用", func(t *testing.T) {
		_, err := SelectTemplateDir(layout, "nope")
		if err == nil || !strings.Contains(err.Error(), "minimal") {
			t.Fatalf("错误应列出可用模板: %v", err)
		}
	})

	t.Run("单模板指定名报错", func(t *testing.T) {
		single := &SourceLayout{Dir: dir}
		if _, err := SelectTemplateDir(single, "x"); err == nil {
			t.Fatal("期望报错")
		}
	})
}

func TestResolveTemplateDirGit(t *testing.T) {
	ws := testfixture.NewWorkspace(t)

	// 建一个含模板的真实仓库（测试可直接 exec git），再以 file:// url clone
	repo := ws.Mkdir("origin")
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	ws.WriteFile(filepath.Join("origin", "template.yaml"), []byte("version: 1\n"))
	ws.WriteFile(filepath.Join("origin", "main.go"), []byte("__PROJECT__"))
	run("init", "-b", "master")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	run("add", ".")
	run("commit", "-m", "init")

	// clone 后工作区有内容、.git 存在（render 阶段会跳过）、cleanup 可删
	dir, cleanup, err := ResolveTemplateDir("file://" + repo)
	if err != nil {
		t.Fatalf("git 来源解析失败: %v", err)
	}
	if cleanup == nil {
		t.Fatal("git 来源应返回 cleanup")
	}
	if !fileExists(filepath.Join(dir, "template.yaml")) {
		t.Fatal("clone 结果缺少 template.yaml")
	}
	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("cleanup 应删除临时目录")
	}

	t.Run("本地路径不走 git", func(t *testing.T) {
		dir, cleanup, err := ResolveTemplateDir(repo)
		if err != nil {
			t.Fatalf("报错: %v", err)
		}
		if dir != repo || cleanup != nil {
			t.Fatalf("本地来源应原样返回: %q cleanup=%v", dir, cleanup != nil)
		}
	})

	t.Run("clone 失败保留现场", func(t *testing.T) {
		_, _, err := ResolveTemplateDir("file:///nonexistent/repo.git")
		if err == nil || !strings.Contains(err.Error(), "保留供排查") {
			t.Fatalf("错误应提示临时目录保留: %v", err)
		}
	})
}

func TestCreateFromCollection(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	ws.WriteFile(filepath.Join("tpls/templates/full/template.yaml"), []byte("version: 1\nvariables:\n  project-name: {prompt: 名, required: true}\npatterns:\n  \"**/*.go\":\n    - {pattern: __PROJECT__, replace: \"${project-name}\"}\n"))
	ws.WriteFile(filepath.Join("tpls/templates/full/main.go"), []byte("package __PROJECT__"))
	ws.WriteFile(filepath.Join("tpls/templates/minimal/template.yaml"), []byte("version: 1\n"))
	ws.WriteFile(filepath.Join("tpls/templates/minimal/x.go"), []byte("x"))

	svc := NewService(config.CreateConfig{})
	target := ws.Join("out")
	if err := svc.Create(ws.Join("tpls"), "full", target, map[string]string{"project-name": "demo"}); err != nil {
		t.Fatalf("Create 报错: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(target, "main.go"))
	if err != nil || string(data) != "package demo" {
		t.Fatalf("main.go = %q, err %v", data, err)
	}
	// 只生成选中子模板的内容
	if _, err := os.Stat(filepath.Join(target, "x.go")); !os.IsNotExist(err) {
		t.Fatal("不应生成未选中模板的文件")
	}
}
