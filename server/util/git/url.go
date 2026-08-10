// Package git 提供 git 相关的通用工具（URL 解析等）。
package git

import (
	"net/url"
	"strings"
)

// RepoUrl 是解析后的远程仓库地址。
type RepoUrl struct {
	Scheme string // git（SSH 形态）/ https / http
	Host   string // 域名，无协议，如 github.com
	Path   string // 路径，含前导 /，如 /heyuuu/cube.git
}

// ParseRepoUrl 解析远程仓库地址，支持两种形态：
//   - git@host:path （SSH）：scheme="git"，path 补前导 / 规范化（与 https 一致，避免下游匹配歧义）。
//   - https://host/path：用 net/url 解析。
//
// 非法输入返回中文错误。
func ParseRepoUrl(rawURL string) (*RepoUrl, error) {
	rawURL = strings.TrimSpace(rawURL)

	// try parse as `git@{host}:{path}`
	if strings.HasPrefix(rawURL, "git@") {
		host, p, _ := strings.Cut(rawURL[4:], ":")
		// 规范化：补上前导 '/'，与 https 解析结果保持一致，
		// 避免下游（如 CloneRule 匹配）按协议表现不一致
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		repoUrl := &RepoUrl{
			Scheme: "git",
			Host:   host,
			Path:   p,
		}
		return repoUrl, nil
	}

	// `https://{host}/{path}`
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	return &RepoUrl{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   u.Path,
	}, nil
}

func (u *RepoUrl) IsSSH() bool { return u.Scheme == "git" }
