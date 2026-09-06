package gitapi

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// githubClient GitHub REST API（官方站 api.github.com，企业版同域）。
type githubClient struct{ baseClient }

type ghRepo struct {
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	CloneUrl      string    `json:"clone_url"`
	DefaultBranch string    `json:"default_branch"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// nsEndpoint 命名空间端点前缀（探测与拉取共用）。
func nsEndpoint(nsType NamespaceType) (string, error) {
	switch nsType {
	case NamespacePersonal:
		return "/users/", nil
	case NamespaceOrg:
		return "/orgs/", nil
	default:
		return "", fmt.Errorf("未知的命名空间类型: %q", nsType)
	}
}

func (c *githubClient) ListNamespaceRepos(ctx context.Context, nsType NamespaceType, path string) ([]RemoteRepo, error) {
	prefix, err := nsEndpoint(nsType)
	if err != nil {
		return nil, err
	}
	var repos []RemoteRepo
	err = listPaged(ctx, func(ctx context.Context, page int) (int, error) {
		var items []ghRepo
		q := url.Values{"per_page": {"100"}, "page": {fmt.Sprint(page)}}
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

func (c *githubClient) DetectNamespace(ctx context.Context, path string) (NamespaceType, error) {
	return detectByProbe(ctx,
		func() error { return c.doGet(ctx, "/users/"+url.PathEscape(path), nil, nil) },
		func() error { return c.doGet(ctx, "/orgs/"+url.PathEscape(path), nil, nil) },
	)
}
