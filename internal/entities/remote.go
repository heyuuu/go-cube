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

func NewHub(name string, host string, defaultPath string) *Remote {
	return &Remote{name: name, host: host, defaultPath: defaultPath}
}

func (h *Remote) Name() string {
	return h.name
}

func (h *Remote) Host() string {
	return h.host
}

func (h *Remote) MapDefaultPath(url *git.RepoUrl) (string, bool) {
	if h.defaultPath == "" {
		return "", false
	}

	repoPath := strings.ReplaceAll(url.Path, ".git", "")
	return filepath.Join(h.defaultPath, repoPath), true
}
