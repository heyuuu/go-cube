package entities

import (
	"github.com/heyuuu/go-cube/internal/config"
	"github.com/heyuuu/go-cube/internal/util/git"
	"path/filepath"
	"strings"
)

type Remote struct {
	name        string // 远端名，唯一标识符
	host        string // 远端域名
	defaultPath string
}

func NewRemote(conf config.RemoteConfig) *Remote {
	return &Remote{
		name:        conf.Name,
		host:        conf.Host,
		defaultPath: conf.DefaultPath,
	}
}

func (r *Remote) Name() string        { return r.name }
func (r *Remote) Host() string        { return r.host }
func (r *Remote) DefaultPath() string { return r.defaultPath }

func (r *Remote) MapDefaultPath(url *git.RepoUrl) (string, bool) {
	if r.defaultPath == "" {
		return "", false
	}

	repoPath := strings.ReplaceAll(url.Path, ".git", "")
	return filepath.Join(r.defaultPath, repoPath), true
}
