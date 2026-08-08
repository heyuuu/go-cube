package project

import (
	"testing"
)

func TestMatchCloneRule(t *testing.T) {
	// LocalPath 使用中性占位路径，避免暴露开发者本地环境
	rules := []CloneRule{
		{RepoHost: "github.com", RepoPrefix: "/heyuuu", LocalPath: "/home/dev/projects/heyuuu"},
		{RepoHost: "github.com", RepoPrefix: "/", LocalPath: "/home/dev/projects/github"},
		{RepoHost: "gitee.com", RepoPrefix: "/heyuuu", LocalPath: "/home/dev/projects/gitee-heyuuu"},
	}

	tests := []struct {
		name          string
		repoUrl       string
		wantOk        bool
		wantRule      CloneRule
		wantLocalPath string
	}{
		{
			name:          "https 长前缀命中",
			repoUrl:       "https://github.com/heyuuu/cube.git",
			wantOk:        true,
			wantRule:      rules[0],
			wantLocalPath: "/home/dev/projects/heyuuu/cube",
		},
		{
			// BUG 复现：同一份配置对 ssh url 也应命中
			name:          "ssh 长前缀命中（协议无关）",
			repoUrl:       "git@github.com:heyuuu/cube.git",
			wantOk:        true,
			wantRule:      rules[0],
			wantLocalPath: "/home/dev/projects/heyuuu/cube",
		},
		{
			name:          "https 根前缀回退",
			repoUrl:       "https://github.com/somebody/other.git",
			wantOk:        true,
			wantRule:      rules[1],
			wantLocalPath: "/home/dev/projects/github/somebody/other",
		},
		{
			name:          "ssh 根前缀回退",
			repoUrl:       "git@github.com:somebody/other.git",
			wantOk:        true,
			wantRule:      rules[1],
			wantLocalPath: "/home/dev/projects/github/somebody/other",
		},
		{
			name:          "多 host 命中 gitee (https)",
			repoUrl:       "https://gitee.com/heyuuu/foo.git",
			wantOk:        true,
			wantRule:      rules[2],
			wantLocalPath: "/home/dev/projects/gitee-heyuuu/foo",
		},
		{
			name:          "多 host 命中 gitee (ssh)",
			repoUrl:       "git@gitee.com:heyuuu/foo.git",
			wantOk:        true,
			wantRule:      rules[2],
			wantLocalPath: "/home/dev/projects/gitee-heyuuu/foo",
		},
		{
			name:     "host 不匹配",
			repoUrl:  "https://gitlab.com/heyuuu/x.git",
			wantOk:   false,
			wantRule: CloneRule{},
		},
		{
			name:     "非法 url",
			repoUrl:  "://bad-url",
			wantOk:   false,
			wantRule: CloneRule{},
		},
		{
			name:          "长前缀优先于短前缀",
			repoUrl:       "https://github.com/heyuuu/deep/repo.git",
			wantOk:        true,
			wantRule:      rules[0],
			wantLocalPath: "/home/dev/projects/heyuuu/deep/repo",
		},
		{
			name:          "无 .git 后缀的 url 也能匹配",
			repoUrl:       "https://github.com/heyuuu/plain",
			wantOk:        true,
			wantRule:      rules[0],
			wantLocalPath: "/home/dev/projects/heyuuu/plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRule, gotLocalPath, gotOk := MatchCloneRule(tt.repoUrl, rules)

			if gotOk != tt.wantOk {
				t.Fatalf("MatchCloneRule(%q) ok = %v, want %v", tt.repoUrl, gotOk, tt.wantOk)
			}
			if !tt.wantOk {
				return
			}
			if gotRule != tt.wantRule {
				t.Errorf("MatchCloneRule(%q) rule = %+v, want %+v", tt.repoUrl, gotRule, tt.wantRule)
			}
			if gotLocalPath != tt.wantLocalPath {
				t.Errorf("MatchCloneRule(%q) localPath = %q, want %q", tt.repoUrl, gotLocalPath, tt.wantLocalPath)
			}
		})
	}
}
