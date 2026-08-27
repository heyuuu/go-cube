package iconkit

import "testing"

func TestValidateIcon(t *testing.T) {
	cases := []struct {
		name    string
		icon    *Icon
		wantErr bool
	}{
		{"nil 合法（可选字段）", nil, false},
		{"lucide 合法", &Icon{Type: IconTypeLucide, Value: "folder-open"}, false},
		{"image 合法", &Icon{Type: IconTypeImage, Value: "aGVsbG8="}, false},
		{"未知 type 报错", &Icon{Type: "svg", Value: "x"}, true},
		{"value 为空报错", &Icon{Type: IconTypeLucide, Value: ""}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := ValidateIcon(c.icon); c.wantErr != (err != nil) {
				t.Fatalf("wantErr=%v, err=%v", c.wantErr, err)
			}
		})
	}
}
