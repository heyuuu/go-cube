// Package gitapi 提供 git 托管平台的 REST API 客户端（github / gitea / gitee）。
// 与 util/git 成对：git 管本机 git 子进程，gitapi 管平台 HTTP API。
// 当前只做一件事：通过 account（token）拉取认证账号名下的全部仓库（含私有库、
// 所属组织仓库）。刻意不做按命名空间列仓库——各平台该能力端点分裂且部分不可靠
// （github/gitee 的 /users/{path}/repos 只回公开库；org 列表会混入无权限的幽灵条目），
// 认证账号端点 /user/repos 是唯一「列表即真相」的口径（提案 1044）。
//
// util 纪律：只依赖入参（host / token / http client）做网络调用，不读进程状态与环境；
// 对外暴露统一类型（RemoteRepo），不含业务实体，业务映射归调用方。
package gitapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RemoteRepo 平台 API 返回的远端仓库统一形态（跨方言归一后）。
type RemoteRepo struct {
	Name          string    `json:"name"`          // 仓库短名，如 cube
	FullName      string    `json:"fullName"`      // 全名（含命名空间前缀），如 heyuuu/cube
	CloneUrl      string    `json:"cloneUrl"`      // 克隆地址（ssh / https 任一形态，对账时统一归一化）
	DefaultBranch string    `json:"defaultBranch"` // 默认分支名
	UpdatedAt     time.Time `json:"updatedAt"`     // 最近更新时间
}

// Client 单个 forge 平台的 API 客户端（按 kind 分发的统一接口）。
type Client interface {
	// ListAccountRepos 拉取认证账号名下的全部仓库（含私有库与所属组织仓库，分页拉全量）。
	// token 为空时认证端点不可用，返回 ErrUnauthorized。
	ListAccountRepos(ctx context.Context) ([]RemoteRepo, error)
}

// ErrUnauthorized token 无效或未提供（认证端点 401/403 的统一口径）。
var ErrUnauthorized = errors.New("账号凭证无效或未提供，无法拉取仓库列表")

// NewClient 按 kind 构造对应方言的客户端。
// host 为平台域名（可带端口）；github / gitee 有官方默认域名，gitea 必须显式给（自建场景）。
// token 必填（只走认证端点）。httpClient 传 nil 用默认（带超时）。
func NewClient(kind, host, token string, httpClient *http.Client) (Client, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrUnauthorized
	}
	host = strings.TrimSpace(host)
	switch kind {
	case "github":
		// 官方站 API 在 api.github.com；自填其他 host 视为 GitHub Enterprise，API 同域
		apiHost := host
		if host == "" || host == "github.com" {
			apiHost = "api.github.com"
		}
		c := &githubClient{baseClient{apiHost: apiHost, token: token, http: httpClient}}
		c.authHeaderFn = func(req *http.Request) {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return c, nil
	case "gitea":
		if host == "" {
			return nil, fmt.Errorf("gitea 平台必须指定 host（自建场景无默认域名）")
		}
		c := &giteaClient{baseClient{apiHost: host, token: token, http: httpClient}}
		c.authHeaderFn = func(req *http.Request) {
			req.Header.Set("Authorization", "token "+token)
		}
		return c, nil
	case "gitee":
		if host == "" {
			host = "gitee.com"
		}
		c := &giteeClient{baseClient{apiHost: host, token: token, http: httpClient}}
		// gitee 的认证走 query 参数 access_token（其 OpenAPI 不认 Authorization header）；
		// 必须走 authQueryFn 在 URL 构造前注入，header 阶段为时已晚（1044 修的时序 bug）
		c.authQueryFn = func(q url.Values) {
			q.Set("access_token", token)
		}
		return c, nil
	default:
		return nil, fmt.Errorf("gitapi 不支持的平台 kind: %q", kind)
	}
}
