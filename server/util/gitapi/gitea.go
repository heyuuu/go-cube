package gitapi

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// giteaClient Gitea API v1（/api/v1 前缀，自建场景为主）。token 走 `Authorization: token` header。
type giteaClient struct{ baseClient }

type giteaRepo struct {
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	CloneUrl      string    `json:"clone_url"`
	DefaultBranch string    `json:"default_branch"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (c *giteaClient) ListNamespaceRepos(ctx context.Context, nsType NamespaceType, path string) ([]RemoteRepo, error) {
	prefix, err := nsEndpoint(nsType)
	if err != nil {
		return nil, err
	}
	prefix = "/api/v1" + prefix
	var repos []RemoteRepo
	err = listPaged(ctx, func(ctx context.Context, page int) (int, error) {
		var items []giteaRepo
		q := url.Values{"limit": {"100"}, "page": {fmt.Sprint(page)}}
		if err := c.doGet(ctx, prefix+url.PathEscape(path)+"/repos", q, &items); err != nil {
			return 0, err
		}
		for _, r := range items {
			repos = append(repos, RemoteRepo{
				Name:          r.Name,
				FullName:      r.FullName,
				CloneUrl:      r.CloneUrl,
				DefaultBranch: r.DefaultBranch,
				UpdatedAt:     r.UpdatedAt,
			})
		}
		return len(items), nil
	})
	return repos, err
}

func (c *giteaClient) DetectNamespace(ctx context.Context, path string) (NamespaceType, error) {
	return detectByProbe(ctx,
		func() error { return c.doGet(ctx, "/api/v1/users/"+url.PathEscape(path), nil, nil) },
		func() error { return c.doGet(ctx, "/api/v1/orgs/"+url.PathEscape(path), nil, nil) },
	)
}
