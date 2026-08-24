package create

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// globGroup 是一个 glob 分组及其替换规则（来自 patterns 的一个 key）。
type globGroup struct {
	glob  string
	rules []ReplaceRule
}

// compileGroups 把 patterns 编译为按 glob 字典序排序的分组列表，
// 并提前校验 glob 语法与 pattern 非空——模板错误必须在生成任何文件之前报出。
func compileGroups(tpl *TemplateYaml) ([]globGroup, error) {
	globs := make([]string, 0, len(tpl.Patterns))
	for g := range tpl.Patterns {
		globs = append(globs, g)
	}
	sort.Strings(globs) // map 遍历无序，排序保证多次执行结果一致

	groups := make([]globGroup, 0, len(globs))
	for _, g := range globs {
		if !doublestar.ValidatePattern(g) {
			return nil, fmt.Errorf("patterns 含非法 glob: %s", g)
		}
		rules := tpl.Patterns[g]
		for _, r := range rules {
			if r.Pattern == "" {
				return nil, fmt.Errorf("glob %q 下存在空 pattern 的替换规则", g)
			}
		}
		groups = append(groups, globGroup{glob: g, rules: rules})
	}
	return groups, nil
}

// applyRules 按声明顺序逐条应用精确字符串替换（非正则）。
func applyRules(s string, rules []ReplaceRule) string {
	for _, r := range rules {
		s = strings.ReplaceAll(s, r.Pattern, r.Replace)
	}
	return s
}

// Render 遍历模板目录，把所有文件（路径与内容统一替换后）生成到目标目录。
// template.yaml 本身与 .git 不参与生成（.git 预留给 git 来源，本地来源一般也没有）。
// 返回生成的文件数。
func Render(templateDir, targetDir string, tpl *TemplateYaml) (int, error) {
	groups, err := compileGroups(tpl)
	if err != nil {
		return 0, err
	}

	count := 0
	err = filepath.WalkDir(templateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		base := filepath.Base(rel)
		if base == ".git" || base == "template.yaml" && !strings.ContainsRune(rel, '/') {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		// 收集命中该文件的所有分组，路径（含文件名）与内容应用同一组规则
		rules := collectRules(groups, rel)

		outRel := applyRules(rel, rules)
		outPath := filepath.Join(targetDir, filepath.FromSlash(outRel))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("读取模板文件失败: %w", err)
		}
		if !isBinary(data) { // 二进制只做路径替换，内容原样
			data = []byte(applyRules(string(data), rules))
		}
		perm := fs.FileMode(0o644)
		if info, err := d.Info(); err == nil {
			perm = info.Mode().Perm()
		}
		if err := os.WriteFile(outPath, data, perm); err != nil {
			return fmt.Errorf("写入文件失败: %w", err)
		}
		count++
		return nil
	})
	if err != nil {
		return count, fmt.Errorf("生成模板文件失败: %w", err)
	}
	return count, nil
}

// collectRules 返回 glob 命中 rel 的所有分组的规则，按分组的 glob 字典序拼接。
func collectRules(groups []globGroup, rel string) []ReplaceRule {
	var rules []ReplaceRule
	for _, g := range groups {
		if ok, _ := doublestar.Match(g.glob, rel); ok {
			rules = append(rules, g.rules...)
		}
	}
	return rules
}
