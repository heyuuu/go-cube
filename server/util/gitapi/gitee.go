package gitapi

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// giteeClient Gitee OpenAPI v5（/api/v5 前缀）。token 走 query 参数 access_token（方言差异，见 client.go 的 authQueryFn）。
type giteeClient struct{ baseClient }

type giteeRepo struct {
	Name          string    `json:"path"` // gitee 的 path 即仓库名（name 可能是展示名）
	FullName      string    `json:"full_name"`
	CloneUrl      string    `json:"ssh_url"` // gitee 无统一 clone_url 字段，取 ssh（归一化后与 https 等价）
	DefaultBranch string    `json:"default_branch"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ListAccountRepos 认证账号名下的全部仓库（含私有库与所属组织仓库——gitee 的
// /user/repos 缺省即涵盖有成员关系的组织，每条自带 full_name 归属）。
func (c *giteeClient) ListAccountRepos(ctx context.Context) ([]RemoteRepo, error) {
	var repos []RemoteRepo
	err := listPaged(ctx, func(ctx context.Context, page int) (int, error) {
		var items []giteeRepo
		q := url.Values{"per_page": {"100"}, "page": {fmt.Sprint(page)}}
		if err := c.doGet(ctx, "/api/v5/user/repos", q, &items); err != nil {
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
