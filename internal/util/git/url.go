package git

import (
	"net/url"
	"strings"
)

type RepoUrl url.URL

func ParseRepoUrl(rawURL string) (*RepoUrl, error) {
	rawURL = strings.TrimSpace(rawURL)

	// try parse as `git@{host}:{path}`
	if strings.HasPrefix(rawURL, "git@") {
		host, Path, _ := strings.Cut(rawURL[4:], ":")
		repoUrl := &RepoUrl{
			Scheme: "git",
			Host:   host,
			Path:   Path,
		}
		return repoUrl, nil
	}

	// `https://{host}/{path}`
	u, err := url.Parse(rawURL)
	return (*RepoUrl)(u), err
}

func (u *RepoUrl) IsSSH() bool { return u.Scheme == "git" }
