package entities

import (
	"github.com/heyuuu/go-cube/internal/util/git"
	"path/filepath"
	"strings"
)

type Remote struct {
	name        string
	host        string
	defaultPath string
}

func NewRemote(name string, host string, defaultPath string) *Remote {
	return &Remote{name: name, host: host, defaultPath: defaultPath}
}

func (r *Remote) Name() string {
	return r.name
}

func (r *Remote) Host() string {
	return r.host
}

func (r *Remote) MapDefaultPath(url *git.RepoUrl) (string, bool) {
	if r.defaultPath == "" {
		return "", false
	}

	repoPath := strings.ReplaceAll(url.Path, ".git", "")
	return filepath.Join(r.defaultPath, repoPath), true
}
