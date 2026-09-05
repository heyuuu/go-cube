package forge

import "testing"

func TestNormalizeHost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"  GitHub.COM ", "github.com"},
		{"github.com.", "github.com"},
		{"gitea.example.com:3000", "gitea.example.com:3000"},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeHost(c.in); got != c.want {
			t.Errorf("NormalizeHost(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestValidateHost(t *testing.T) {
	cases := []struct {
		name string
		host string
		ok   bool
	}{
		{"普通域名", "github.com", true},
		{"带端口", "gitea.example.com:3000", true},
		{"空", "", false},
		{"含协议", "https://github.com", false},
		{"含路径", "github.com/heyuuu", false},
		{"含用户", "git@github.com", false},
	}
	for _, c := range cases {
		err := ValidateHost(c.host)
		if (err == nil) != c.ok {
			t.Errorf("%s: ValidateHost(%q) err=%v, want ok=%v", c.name, c.host, err, c.ok)
		}
	}
}

func TestValidKind(t *testing.T) {
	for _, k := range Kinds() {
		if !ValidKind(k) {
			t.Errorf("kind %q 应合法", k)
		}
	}
	if ValidKind("gitlab") {
		t.Errorf("未声明的 kind 不应合法")
	}
}

func TestRepoHost(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"ssh 形态", "git@github.com:heyuuu/cube.git", "github.com"},
		{"https 形态", "https://github.com/heyuuu/cube.git", "github.com"},
		{"http 形态", "http://gitea.example.com:3000/heyuuu/cube.git", "gitea.example.com:3000"},
		{"ssh url 形态", "ssh://git@gitea.example.com:2222/heyuuu/cube.git", "gitea.example.com:2222"},
		{"空", "", ""},
		{"无 host", "not a url at all", ""},
	}
	for _, c := range cases {
		if got := RepoHost(c.in); got != c.want {
			t.Errorf("%s: RepoHost(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestMatchHost(t *testing.T) {
	forges := []Forge{
		{Host: "github.com", Kind: KindGithub},
		{Host: "gitea.example.com:3000", Kind: KindGitea},
	}
	if f := MatchHost(forges, "GitHub.com."); f == nil || f.Kind != KindGithub {
		t.Fatalf("大小写/点号差异应匹配: %v", f)
	}
	if f := MatchHost(forges, "gitea.example.com:3000"); f == nil || f.Kind != KindGitea {
		t.Fatalf("带端口 host 应匹配: %v", f)
	}
	if f := MatchHost(forges, "unknown.com"); f != nil {
		t.Fatalf("未配置 host 不应匹配: %v", f)
	}
}
