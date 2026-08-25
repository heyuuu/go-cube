package create

import (
	"cube/config"

	"os"
	"path/filepath"
	"testing"

	"cube/internal/testfixture"
)

func TestCreateEndToEnd(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	templateDir := makeTemplateDir(t, ws, "tpl")
	ws.WriteFile(filepath.Join("tpl", "template.yaml"), []byte(`version: 1
variables:
  project-name: {prompt: 项目名, required: true}
  module: {prompt: module, required: true}
patterns:
  "**/*.go":
    - {pattern: __MODULE__, replace: "github.com/heyuuu/${module}"}
    - {pattern: __PROJECT__, replace: "${project-name}"}
  "**/*.md":
    - {pattern: __PROJECT__, replace: "${project-name}"}
init:
  - echo ${project-name} > marker.txt
`))
	target := ws.Join("out", "demo")

	svc := NewService(config.CreateConfig{})
	err := svc.Create(templateDir, "", target, map[string]string{
		"project-name": "demo",
		"module":       "demo",
	})
	if err != nil {
		t.Fatalf("Create 报错: %v", err)
	}

	// init 命令已插值执行（cwd 为生成目录）
	marker, err := os.ReadFile(filepath.Join(target, "marker.txt"))
	if err != nil {
		t.Fatalf("init 未执行: %v", err)
	}
	if string(marker) != "demo\n" {
		t.Fatalf("marker 内容 = %q, want %q", marker, "demo\n")
	}

	// 生成的核心文件
	if got, err := os.ReadFile(filepath.Join(target, "demo", "main.go")); err != nil || string(got) != "package main // github.com/heyuuu/demo" {
		t.Fatalf("main.go 生成不符: %q, err=%v", got, err)
	}
}

func TestCreateErrors(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	svc := NewService(config.CreateConfig{})

	t.Run("缺少 template.yaml", func(t *testing.T) {
		empty := ws.Mkdir("empty")
		err := svc.Create(empty, "", ws.Join("out1"), nil)
		if err == nil {
			t.Fatal("期望报错")
		}
	})

	t.Run("未声明的变量", func(t *testing.T) {
		tpl := makeTemplateDir(t, ws, "tpl2")
		ws.WriteFile(filepath.Join("tpl2", "template.yaml"), []byte("version: 1\nvariables:\n  a: {prompt: a}\n"))
		err := svc.Create(tpl, "", ws.Join("out2"), map[string]string{"typo": "x"})
		if err == nil {
			t.Fatal("期望报错")
		}
	})

	t.Run("目标目录非空", func(t *testing.T) {
		tpl := makeTemplateDir(t, ws, "tpl3")
		ws.WriteFile(filepath.Join("tpl3", "template.yaml"), []byte("version: 1\n"))
		busy := ws.Mkdir("busy")
		ws.WriteFile(filepath.Join("busy", "x.txt"), []byte("x"))
		err := svc.Create(tpl, "", busy, nil)
		if err == nil {
			t.Fatal("期望报错")
		}
	})

	t.Run("init 失败中止", func(t *testing.T) {
		tpl := makeTemplateDir(t, ws, "tpl4")
		ws.WriteFile(filepath.Join("tpl4", "template.yaml"), []byte("version: 1\ninit:\n  - exit 3\n  - echo ok > after.txt\n"))
		target := ws.Join("out4")
		if err := svc.Create(tpl, "", target, nil); err == nil {
			t.Fatal("期望 init 失败报错")
		}
		if _, err := os.Stat(filepath.Join(target, "after.txt")); !os.IsNotExist(err) {
			t.Fatal("init 失败后不应继续执行后续命令")
		}
	})
}
