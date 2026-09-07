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
// 认证钩子分两阶段（不靠方法覆写——Go 的方法提升不会走到外层定义）：
// authQueryFn 在 URL 构造前改写 query（gitee 的 access_token 是 query 参数，晚了就发不出去）；
// authHeaderFn 在请求构造后注入 header（github Bearer / gitea token）。
type baseClient struct {
	apiHost string // 实际发请求的 netloc（github 是 api.github.com 而非配置的 github.com）
	token   string
	http    *http.Client

	authQueryFn  func(q url.Values)
	authHeaderFn func(req *http.Request)

	schemeMu    sync.Mutex
	scheme      string // 首次请求探明后记忆（自建 http 站点避免每次都白试一次 https）
	schemeTried bool
}

// doGet 发起 GET 并把 2xx 响应体解码到 out；401/403 统一映射 ErrUnauthorized。
// scheme 自适应：首次用 https，连接层失败（自建 http 站点）降级 http 重试一次并记忆。
func (c *baseClient) doGet(ctx context.Context, path string, query url.Values, out any) error {
	try := func(scheme string) error {
		if c.authQueryFn != nil {
			c.authQueryFn(query)
		}
		u := scheme + "://" + c.apiHost + path
		if len(query) > 0 {
			u += "?" + query.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return fmt.Errorf("构造请求失败: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if c.authHeaderFn != nil {
			c.authHeaderFn(req)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return &retryableError{err}
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return ErrUnauthorized
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
		// 连接层失败才降级重试；HTTP 层错误（401/500）不重试
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

// listPaged 分页拉全量：page 从 1 起、每页固定条数，短页即停。
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
