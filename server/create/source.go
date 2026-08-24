package create

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cube/util/git"
	"cube/util/pathkit"
	"cube/util/tui"
)

// gitUrlPrefixes 触发 git clone 的来源前缀。本地路径（含 ~/ 开头）不在此列。
var gitUrlPrefixes = []string{"https://", "http://", "git://", "ssh://", "file://", "git@"}

func isGitSource(source string) bool {
	if strings.HasSuffix(source, ".git") {
		return true
	}
	for _, p := range gitUrlPrefixes {
		if strings.HasPrefix(source, p) {
			return true
		}
	}
	return false
}

// ResolveTemplateDir 把来源（本地目录或 git url）解析为「来源根目录」。
// git 来源 clone --depth 1 到系统临时目录，返回 cleanup 供成功后删除临时目录；
// 失败时 cleanup 为 nil（现场保留，路径已在错误信息中给出，便于排查模板问题）。
func ResolveTemplateDir(source string) (dir string, cleanup func(), err error) {
	if !isGitSource(source) {
		absDir, err := pathkit.AbsPath(source)
		if err != nil {
			return "", nil, fmt.Errorf("解析模板来源路径失败: %w", err)
		}
		if info, err := os.Stat(absDir); err != nil || !info.IsDir() {
			return "", nil, fmt.Errorf("模板来源目录不存在或不是目录: %s", pathkit.PrettyPath(absDir))
		}
		return absDir, nil, nil
	}

	tempDir, err := os.MkdirTemp("", "cube-create-")
	if err != nil {
		return "", nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	if err := git.Clone(tempDir, source, 1, ""); err != nil {
		return "", nil, fmt.Errorf("clone 模板仓库失败（临时目录 %s 保留供排查）: %w", tempDir, err)
	}
	return tempDir, func() { _ = os.RemoveAll(tempDir) }, nil
}

// SourceLayout 描述来源根目录的形态：单模板或模板集。
type SourceLayout struct {
	Dir           string   // 来源根目录（绝对路径）
	TemplateNames []string // 非空表示模板集（一级子目录名，字典序）；空表示根目录即单模板
}

// InspectSource 判定来源目录形态：根目录有 template.yaml → 单模板；
// 否则扫一级子目录找 template.yaml → 模板集（只扫一级，嵌套模板集无场景）；两者都不是 → 报错。
func InspectSource(dir string) (*SourceLayout, error) {
	if fileExists(filepath.Join(dir, "template.yaml")) {
		return &SourceLayout{Dir: dir}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取模板来源目录失败: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if fileExists(filepath.Join(dir, e.Name(), "template.yaml")) {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%s 不是合法模板来源：根目录及一级子目录均无 template.yaml", pathkit.PrettyPath(dir))
	}
	sort.Strings(names)
	return &SourceLayout{Dir: dir, TemplateNames: names}, nil
}

// SelectTemplateDir 在来源内定位最终的模板目录：
// 单模板 + 未指定名 → 根目录；单模板 + 指定名 → 报错（防误用）；
// 模板集 + 指定名 → 校验存在（不存在则列出可用名）；模板集 + 未指定名 → 交互选择。
func SelectTemplateDir(layout *SourceLayout, name string) (string, error) {
	if len(layout.TemplateNames) == 0 {
		if name != "" {
			return "", fmt.Errorf("该来源是单模板（根目录即 template.yaml），无需指定模板名 %q", name)
		}
		return layout.Dir, nil
	}
	if name == "" {
		selected, err := tui.SelectItem("选择模板", layout.TemplateNames, func(n string) string { return n })
		if err != nil {
			return "", err
		}
		return filepath.Join(layout.Dir, selected), nil
	}
	for _, n := range layout.TemplateNames {
		if n == name {
			return filepath.Join(layout.Dir, n), nil
		}
	}
	return "", fmt.Errorf("模板 %q 不存在，可用模板: %v", name, layout.TemplateNames)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
