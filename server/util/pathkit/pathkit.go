package pathkit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveAbs(p string, baseDir string) (string, error) {
	// 不变量断言：baseDir 只能是 "" 或绝对路径（调用域全在包内，违反即包内代码 bug）。
	// 代码 bug 优先于业务数据错误暴露，避免被下面的判空掩盖。
	if baseDir != "" && !filepath.IsAbs(baseDir) {
		return "", fmt.Errorf("baseDir 必须为绝对路径或为空: %s", baseDir)
	}
	// 判空
	if p == "" {
		return "", errors.New("路径不可为空")
	}

	// 绝对路径，baseDir 不参与
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}

	// 支持 ~ 前缀
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("获取 home 路径失败: %w", err)
		} else if home == "" {
			return "", errors.New("home 路径为空")
		}
		// TrimPrefix 后 "~" 得 ""（Join 返回 home 本身），两种形态统一处理
		return filepath.Join(home, strings.TrimPrefix(p, "~")), nil
	}

	// 显式相对路径语法：. / .. / ./x / ../x，基于 baseDir 解析。
	// .abc 这类以点开头的文件名不算——那是裸相对，和 sub/x 一样说明调用方传错，
	// 报错而非猜测拼接（cmd 层靠前缀区分 path/name 两种 query 模式，裸相对走 name 分支）。
	if baseDir != "" && (p == "." || p == ".." || strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../")) {
		return filepath.Join(baseDir, p), nil
	}

	// 无 baseDir 时相对路径无处解析
	return "", fmt.Errorf("路径必须是绝对路径、~ 前缀或显式相对路径(./ ../): %s", p)
}

// StaticAbsPath 解析 path 对应的绝对路径，支持以 ~ 表示 home 路径，不支持绝对路径
func StaticAbsPath(p string) (string, error) {
	return resolveAbs(p, "")
}

// AbsPath 解析 path 对应的绝对路径，支持以 ~ 表示 home 路径，支持 . 或 .. 开头的相对路径
func AbsPath(p string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取当前目录失败: %w", err)
	}
	return resolveAbs(p, wd)
}

func PrettyPath(path string) string {
	if path == "" {
		return ""
	}
	// 规范化冗余分隔符 / . / ..，保证后续比较稳定
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	rel, err := filepath.Rel(home, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}

	// path 即 home 本身（rel 为 "."），标准化为 "~"
	if rel == "." {
		return "~"
	}

	return "~/" + rel
}

// CommonPrefix 求一组路径的最长公共前缀目录。
//
// 规则：
//   - 空切片 → ""
//   - 非空 → 所有路径的最长公共前缀；无公共时返回 ""
//
// 单条路径返回其自身（Clean 后），不取父目录——本函数只做前缀运算。
// 输入路径会先经 filepath.Clean 规范化（处理冗余分隔符 / . / ..）。
// 绝对/相对路径无需显式区分：按目录分隔符逐段比对，首字符不同即视为无公共。
func CommonPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	// 入口统一 Clean，后续比较无需再处理脏输入
	for i, p := range paths {
		paths[i] = filepath.Clean(p)
	}

	common := paths[0]
	for _, p := range paths[1:] {
		common = commonPrefixDir(common, p)
		if common == "" {
			return ""
		}
	}
	return common
}

// commonPrefixDir 求两条（已 Clean 的）路径的最长公共目录前缀。
//
// 约定 a 为较短者（入口已交换），故循环中无需再校验 b 的长度边界。
// 以分隔符为界逐段推进：index 指向 a 中「最近一次确认匹配」的分隔符位置。
// 每轮找 a 的下一个分隔符 next，若 a[:next+1] == b[:next+1] 则推进 index；
// 否则共同前缀止于当前 index。
func commonPrefixDir(a, b string) string {
	// 保证 a 为较短者：后续只需以 a 的分隔符推进，不必处理 a 超出 b 长度的情形
	if len(a) > len(b) {
		a, b = b, a
	}

	const sep = "/"
	index := -1
	for {
		// 找 a 中 index 之后的下一个分隔符位置
		next := strings.Index(a[index+1:], sep)
		if next < 0 {
			break // a 已无更多分隔符
		}
		next += index + 1

		// 对应切片不一致 → 共同前缀止于当前 index
		if a[:next+1] != b[:next+1] {
			break
		}
		index = next
	}

	// a 已无更多分隔符：若 a 整体落在 b 的某段边界（b==a 或 b 以 a/ 开头），
	// 则整个 a 都是公共前缀。
	if strings.HasPrefix(b, a) && (len(b) == len(a) || b[len(a)] == sep[0]) {
		return a
	}

	switch {
	case index < 0:
		return ""
	case index == 0:
		return a[:1] // 仅共一个首字符（绝对路径的 "/"）
	default:
		return a[:index] // 截到最后一个匹配的分隔符之前（不含分隔符）
	}
}

// HasPrefix 判断 path 是否为 parent 或 parent 下的目录
func HasPrefix(path string, parent string) bool {
	path = filepath.Clean(path)
	parent = filepath.Clean(parent)
	return path == parent || strings.HasPrefix(path, parent+string(filepath.Separator))
}
