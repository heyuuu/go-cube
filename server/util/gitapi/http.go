package gitapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// baseClient 三方言共享的执行核：apiHost + token + http client + scheme 自适应。
// 方言差异（认证注入）通过 authFn 钩子注入——不靠方法覆写（Go 的方法提升不会走到外层定义）。
type baseClient struct {
	apiHost string // 实际发请求的 netloc（github 是 api.github.com 而非配置的 github.com）
	token   string
	http    *http.Client
	authFn  func(req *http.Request, q url.Values) // nil = 不认证

	schemeMu    sync.Mutex
	scheme      string // 首次请求探明后记忆（自建 http 站点避免每次都白试一次 https）
	schemeTried bool
}

func (c *baseClient) auth(req *http.Request, q url.Values) {
	if c.authFn != nil {
		c.authFn(req, q)
	}
}

// doGet 发起 GET 并把 2xx 响应体解码到 out；404 返回 ErrNamespaceNotFound（探测/拉取按缺失处理的统一口径）。
// scheme 自适应：首次用 https，连接层失败（自建 http 站点）降级 http 重试一次并记忆。
func (c *baseClient) doGet(ctx context.Context, path string, query url.Values, out any) error {
	try := func(scheme string) error {
		u := scheme + "://" + c.apiHost + path
		if len(query) > 0 {
			u += "?" + query.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return fmt.Errorf("构造请求失败: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		c.auth(req, query)
		resp, err := c.http.Do(req)
		if err != nil {
			return &retryableError{err}
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return ErrNamespaceNotFound
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			return fmt.Errorf("平台 API 返回异常: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		if out == nil {
			return nil
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("解析平台 API 响应失败: %w", err)
		}
		return nil
	}

	scheme := c.currentScheme()
	err := try(scheme)
	var retried *retryableError
	if errors.As(err, &retried) && scheme == "https" {
		// 连接层失败才降级重试；HTTP 层错误（404/500）不重试
		scheme = "http"
		if err = try(scheme); err == nil {
			c.rememberScheme(scheme)
		}
		return err
	}
	if err == nil {
		c.rememberScheme(scheme)
	}
	return err
}

// retryableError 标记连接层失败（可降级 http 重试），不外泄到调用方。
type retryableError struct{ err error }

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

func (c *baseClient) currentScheme() string {
	c.schemeMu.Lock()
	defer c.schemeMu.Unlock()
	if c.scheme == "" {
		return "https"
	}
	return c.scheme
}

func (c *baseClient) rememberScheme(scheme string) {
	c.schemeMu.Lock()
	defer c.schemeMu.Unlock()
	if !c.schemeTried {
		c.schemeTried = true
		c.scheme = scheme
	}
}

// listPaged 分页拉全量：page 从 1 起、per_page 固定，短页即停。
// fetchOne 负责单页请求与解码，返回本页条数（方言差异收敛在 fetchOne，分页循环只有一份）。
func listPaged(ctx context.Context, fetchOne func(ctx context.Context, page int) (int, error)) error {
	const perPage = 100
	for page := 1; ; page++ {
		n, err := fetchOne(ctx, page)
		if err != nil {
			return err
		}
		if n < perPage {
			return nil
		}
	}
}

// detectByProbe 探测命名空间类型的通用实现：先试 personal 端点根，404 再试 org。
// probeRoot 返回某类型命名空间详情端点的根（如 /users/{path}）；响应体不关心，只看 200/404。
func detectByProbe(ctx context.Context, probePersonal, probeOrg func() error) (NamespaceType, error) {
	if err := probePersonal(); err == nil {
		return NamespacePersonal, nil
	}
	if err := probeOrg(); err == nil {
		return NamespaceOrg, nil
	} else if !errors.Is(err, ErrNamespaceNotFound) {
		return "", err // 网络/认证故障与「不存在」要区分开，不能静默判成未找到
	}
	return "", ErrNamespaceNotFound
}
