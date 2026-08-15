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

// WebUrl 把仓库地址转成可在浏览器打开的网页地址（直接拿 host+path 拼 https）。
//
// 适用于 github/gitee 等公开托管平台；自建私服或无法识别 host 时返回空串。
// 网络协议展示统一走 https——网页访问通常也是 https，无需区分 ssh/http。
func (u *RepoUrl) WebUrl() string {
	if u.Host == "" || u.Path == "" {
		return ""
	}
	path := strings.TrimSuffix(u.Path, ".git")
	return "https://" + u.Host + "/" + strings.TrimPrefix(path, "/")
}
