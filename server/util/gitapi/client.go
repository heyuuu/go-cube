// Package gitapi 提供 git 托管平台的 REST API 客户端（github / gitea / gitee）。
// 与 util/git 成对：git 管本机 git 子进程，gitapi 管平台 HTTP API（拉取命名空间下的
// 仓库列表、探测命名空间类型）。token 只作 API 认证，从不参与 git 传输。
//
// util 纪律：只依赖入参（host / token / http client）做网络调用，不读进程状态与环境；
// 对外暴露统一类型（RemoteRepo 等），不含业务实体，业务映射归调用方。
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

// NamespaceType 命名空间类型，决定拉取端点（users vs orgs）。
type NamespaceType string

const (
	NamespacePersonal NamespaceType = "personal" // 个人空间
	NamespaceOrg      NamespaceType = "org"      // 组织空间
)

// ValidNamespaceType 判断类型是否合法。
func ValidNamespaceType(t NamespaceType) bool {
	return t == NamespacePersonal || t == NamespaceOrg
}

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
	// ListNamespaceRepos 拉取命名空间下全部仓库（分页拉全量）。
	ListNamespaceRepos(ctx context.Context, nsType NamespaceType, path string) ([]RemoteRepo, error)
	// DetectNamespace 探测 path 是个人空间还是组织空间；不存在/不可访问返回错误。
	DetectNamespace(ctx context.Context, path string) (NamespaceType, error)
}

// ErrNamespaceNotFound 命名空间不存在或不可访问（探测/拉取共用的语义化错误）。
var ErrNamespaceNotFound = errors.New("命名空间不存在或不可访问")

// NewClient 按 kind 构造对应方言的客户端。
// host 为平台域名（可带端口）；github / gitee 有官方默认域名，gitea 必须显式给（自建场景）。
// token 可为空（匿名，仅公开数据）。httpClient 传 nil 用默认（带超时）。
func NewClient(kind, host, token string, httpClient *http.Client) (Client, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
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
		c.authFn = func(req *http.Request, _ url.Values) {
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		}
		return c, nil
	case "gitea":
		if host == "" {
			return nil, fmt.Errorf("gitea 平台必须指定 host（自建场景无默认域名）")
		}
		c := &giteaClient{baseClient{apiHost: host, token: token, http: httpClient}}
		c.authFn = func(req *http.Request, _ url.Values) {
			if token != "" {
				req.Header.Set("Authorization", "token "+token)
			}
		}
		return c, nil
	case "gitee":
		if host == "" {
			host = "gitee.com"
		}
		c := &giteeClient{baseClient{apiHost: host, token: token, http: httpClient}}
		// gitee 的认证走 query 参数 access_token（其 OpenAPI 不认 Authorization header）
		c.authFn = func(_ *http.Request, q url.Values) {
			if token != "" {
				q.Set("access_token", token)
			}
		}
		return c, nil
	default:
		return nil, fmt.Errorf("gitapi 不支持的平台 kind: %q", kind)
	}
}
