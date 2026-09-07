package gitapi

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// githubClient GitHub REST API（官方站 api.github.com，企业版同域）。token 走 Bearer header。
type githubClient struct{ baseClient }

type ghRepo struct {
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	CloneUrl      string    `json:"clone_url"`
	DefaultBranch string    `json:"default_branch"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ListAccountRepos 认证账号名下的全部仓库。affiliation 缺省即
// owner,collaborator,organization_member——个人私有库 + 所属组织 + 被协作的仓库。
func (c *githubClient) ListAccountRepos(ctx context.Context) ([]RemoteRepo, error) {
	var repos []RemoteRepo
	err := listPaged(ctx, func(ctx context.Context, page int) (int, error) {
		var items []ghRepo
		q := url.Values{"per_page": {"100"}, "page": {fmt.Sprint(page)}}
		if err := c.doGet(ctx, "/user/repos", q, &items); err != nil {
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
