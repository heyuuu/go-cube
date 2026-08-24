package create

import (
	"bytes"
	"fmt"
	"regexp"
)

// varRefPattern 匹配 ${var} 形式的变量引用，只在 template.yaml 的值内生效。
var varRefPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// interpolate 对字符串做 ${var} → 实际值替换，引用了未收集的变量时报错
// （拼写错误应尽早暴露，而不是静默留在生成结果里）。
func interpolate(s string, vars map[string]string) (string, error) {
	var unknown []string
	out := varRefPattern.ReplaceAllStringFunc(s, func(ref string) string {
		name := ref[2 : len(ref)-1]
		if v, ok := vars[name]; ok {
			return v
		}
		unknown = append(unknown, name)
		return ref
	})
	if len(unknown) > 0 {
		return "", fmt.Errorf("引用了未定义的变量: %v", unknown)
	}
	return out, nil
}

// isBinary 粗判二进制文件：内容含 NUL 字节即视为二进制，内容不做替换原样复制。
func isBinary(data []byte) bool { return bytes.IndexByte(data, 0) >= 0 }
