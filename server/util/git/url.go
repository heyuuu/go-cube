package git

import (
	"net/url"
	"strings"
)

type RepoUrl struct {
	Scheme string
	Host   string
	Path   string
}

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
