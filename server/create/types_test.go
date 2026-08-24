package create

import (
	"strings"
	"testing"
)

func TestInitTemplateYaml(t *testing.T) {
	valid := `
version: 1
variables:
  project-name:
    prompt: 项目名
    required: true
  author:
    prompt: 作者
    default: heyu
patterns:
  "**/*.go":
    - pattern: __MODULE__
      replace: github.com/${author}/${project-name}
init:
  - git init -b master
`
	tpl, err := InitTemplateYaml([]byte(valid))
	if err != nil {
		t.Fatalf("解析合法 yaml 报错: %v", err)
	}
	if tpl.Version != 1 {
		t.Fatalf("version = %d, want 1", tpl.Version)
	}
	if len(tpl.Variables) != 2 || !tpl.Variables["project-name"].Required {
		t.Fatalf("variables 解析结果不符: %+v", tpl.Variables)
	}
	if tpl.Variables["author"].Default != "heyu" {
		t.Fatalf("default 解析失败: %+v", tpl.Variables["author"])
	}
	if got := tpl.Patterns["**/*.go"][0].Replace; got != "github.com/${author}/${project-name}" {
		t.Fatalf("replace 解析失败: %q", got)
	}
	if len(tpl.Init) != 1 || tpl.Init[0] != "git init -b master" {
		t.Fatalf("init 解析失败: %+v", tpl.Init)
	}

	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{"缺少 version", "variables: {}", "version"},
		{"版本过高", "version: 99", "高于引擎支持"},
		{"坏缩进", "version: 1\nvariables:\n  a:\n   b:\n   c: 1\n  d", "解析失败"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := InitTemplateYaml([]byte(tt.yaml))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("期望错误含 %q, got %v", tt.wantErr, err)
			}
		})
	}

	t.Run("未知字段忽略", func(t *testing.T) {
		_, err := InitTemplateYaml([]byte("version: 1\nfuture-field: whatever\n"))
		if err != nil {
			t.Fatalf("未知字段应忽略: %v", err)
		}
	})
}
