package cubefile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cube/internal/testfixture"
)

func writeFile(t *testing.T, root, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".cube"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cube", "cube.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// 存在性语义：空数组也是显式声明（WorkspacesSet=true），字段缺失才是「未声明」。
func TestLoadPresence(t *testing.T) {
	root := testfixture.NewWorkspace(t).Dir

	writeFile(t, root, `{"workspaces":[]}`)
	f, ok := Load(root)
	if !ok || !f.WorkspacesSet || len(f.Workspaces) != 0 {
		t.Fatalf("空数组应是显式声明: ok=%v %+v", ok, f)
	}

	writeFile(t, root, `{"workspaceScanRule":"pnpm"}`)
	f, ok = Load(root)
	if !ok || f.WorkspacesSet || f.WorkspaceScanRule != "pnpm" {
		t.Fatalf("字段缺失应 WorkspacesSet=false: ok=%v %+v", ok, f)
	}

	// 坏 JSON / 文件不存在 → (nil,false)，降级语义由调用方决定
	writeFile(t, root, `{bad`)
	if _, ok := Load(root); ok {
		t.Fatal("坏 JSON 应返回 false")
	}
	if _, ok := Load(root + "-absent"); ok {
		t.Fatal("文件不存在应返回 false")
	}
}

// Save 保留未知节：workspace 节外未来的节不许被写丢；已知节整体覆盖
// （scanRule 空则清掉）；空数组声明写盘后仍读得回 WorkspacesSet=true。
func TestSavePreservesUnknownSections(t *testing.T) {
	root := testfixture.NewWorkspace(t).Dir
	writeFile(t, root, `{"workspaceScanRule":"pnpm","futureSection":{"a":1}}`)

	if err := Save(root, &File{Workspaces: []Declared{{Name: "s", Path: "server"}}, WorkspacesSet: true}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".cube", "cube.json"))
	out := string(data)
	if !strings.Contains(out, `"futureSection"`) {
		t.Fatalf("未知节应保留: %s", out)
	}
	if strings.Contains(out, "workspaceScanRule") {
		t.Fatalf("scanRule 应被覆盖清掉: %s", out)
	}
	f, ok := Load(root)
	if !ok || !f.WorkspacesSet || len(f.Workspaces) != 1 || f.Workspaces[0].Path != "server" {
		t.Fatalf("写读往返不符: ok=%v %+v", ok, f)
	}

	// 空数组声明（显式「无 workspace」）落盘往返不丢
	if err := Save(root, &File{Workspaces: []Declared{}, WorkspacesSet: true}); err != nil {
		t.Fatal(err)
	}
	f, ok = Load(root)
	if !ok || !f.WorkspacesSet || len(f.Workspaces) != 0 {
		t.Fatalf("空数组声明往返应保持 WorkspacesSet=true: ok=%v %+v", ok, f)
	}
	if !strings.Contains(string(mustRead(t, root)), `"workspaces": []`) {
		t.Fatalf("空数组应显式写出: %s", mustRead(t, root))
	}
}

func mustRead(t *testing.T, root string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".cube", "cube.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
