package create

import (
	"testing"
)

func TestInterpolate(t *testing.T) {
	vars := map[string]string{"name": "my-app", "author": "heyu"}
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"无引用", "plain text", "plain text", false},
		{"单变量", "${name}", "my-app", false},
		{"多变量混排", "github.com/${author}/${name}", "github.com/heyu/my-app", false},
		{"同一变量多次", "${name}/${name}.go", "my-app/my-app.go", false},
		{"未闭合不算引用", "${name", "${name", false},
		{"未定义变量", "${nope}", "", true},
		{"混合未定义", "${author}/${nope}", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolate(tt.input, vars)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("期望报错，得到 %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("意外报错: %v", err)
			}
			if got != tt.want {
				t.Fatalf("interpolate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsBinary(t *testing.T) {
	if isBinary([]byte("hello")) {
		t.Fatal("纯文本被误判为二进制")
	}
	if !isBinary([]byte("a\x00b")) {
		t.Fatal("含 NUL 字节未被判定为二进制")
	}
}
