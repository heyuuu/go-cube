package git

import (
	"reflect"
	"testing"
)

func TestParseRepoUrl(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		want    *RepoUrl
		wantErr bool
	}{
		// --- https ---
		{
			name:   "https 标准地址",
			rawURL: "https://github.com/heyuuu/cube.git",
			want:   &RepoUrl{Scheme: "https", Host: "github.com", Path: "/heyuuu/cube.git"},
		},
		{
			name:   "https 带端口",
			rawURL: "https://git.internal.local:8080/team/repo.git",
			want:   &RepoUrl{Scheme: "https", Host: "git.internal.local:8080", Path: "/team/repo.git"},
		},
		{
			name:   "https 无 .git 后缀",
			rawURL: "https://github.com/heyuuu/plain",
			want:   &RepoUrl{Scheme: "https", Host: "github.com", Path: "/heyuuu/plain"},
		},

		// --- ssh (git@host:path) ---
		{
			name:   "ssh 标准地址（路径无前导斜杠，需规范化）",
			rawURL: "git@github.com:heyuuu/cube.git",
			want:   &RepoUrl{Scheme: "git", Host: "github.com", Path: "/heyuuu/cube.git"},
		},
		{
			name:   "ssh 路径已带前导斜杠（不应出现双斜杠）",
			rawURL: "git@github.com:/heyuuu/cube.git",
			want:   &RepoUrl{Scheme: "git", Host: "github.com", Path: "/heyuuu/cube.git"},
		},
		{
			name:   "ssh 多级路径",
			rawURL: "git@gitee.com:org/sub/repo.git",
			want:   &RepoUrl{Scheme: "git", Host: "gitee.com", Path: "/org/sub/repo.git"},
		},

		// --- 其它 ---
		{
			name:   "首尾空白会被 TrimSpace",
			rawURL: "  https://github.com/heyuuu/cube.git  ",
			want:   &RepoUrl{Scheme: "https", Host: "github.com", Path: "/heyuuu/cube.git"},
		},
		{
			name:    "非法 url 应报错",
			rawURL:  "://bad-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRepoUrl(tt.rawURL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseRepoUrl(%q) error = %v, wantErr %v", tt.rawURL, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRepoUrl(%q) = %+v, want %+v", tt.rawURL, got, tt.want)
			}
		})
	}
}

func TestRepoUrl_IsSSH(t *testing.T) {
	tests := []struct {
		name string
		u    *RepoUrl
		want bool
	}{
		{"git 协议是 ssh", &RepoUrl{Scheme: "git", Host: "github.com"}, true},
		{"https 协议非 ssh", &RepoUrl{Scheme: "https", Host: "github.com"}, false},
		{"http 协议非 ssh", &RepoUrl{Scheme: "http", Host: "github.com"}, false},
		{"空 scheme 非 ssh", &RepoUrl{Host: "github.com"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.u.IsSSH(); got != tt.want {
				t.Errorf("IsSSH() = %v, want %v", got, tt.want)
			}
		})
	}
}
