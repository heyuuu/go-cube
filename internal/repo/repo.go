package repo

import (
	"go-cube/internal/git"
	"path/filepath"
	"strings"
)

type Repo interface {
	Url() string
}

type Hub struct {
	name        string
	host        string
	defaultPath string
}

func NewHub(name string, host string, defaultPath string) *Hub {
	return &Hub{name: name, host: host, defaultPath: defaultPath}
}

func (h *Hub) Name() string {
	return h.name
}

func (h *Hub) Host() string {
	return h.host
}

func (h *Hub) MapDefaultPath(url *git.RepoUrl) (string, bool) {
	if h.defaultPath == "" {
		return "", false
	}

	repoPath := strings.ReplaceAll(url.Path, ".git", "")
	return filepath.Join(h.defaultPath, repoPath), true
}
