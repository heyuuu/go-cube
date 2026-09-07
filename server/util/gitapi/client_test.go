package gitapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// newFakeServer 起一个 httptest 服务并返回指向它的 client（host 指向本机，kind 决定方言路径）。
func newFakeServer(t *testing.T, kind, token string, handler http.HandlerFunc) Client {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	u, _ := url.Parse(ts.URL)
	client, err := NewClient(kind, u.Host, token, ts.Client())
	if err != nil {
		t.Fatalf("NewClient(%s) 失败: %v", kind, err)
	}
	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("编码响应失败: %v", err)
	}
}

// TestGithubListAccountRepos 认证端点 /user/repos + token header + 字段映射 + 单页短页即停。
func TestGithubListAccountRepos(t *testing.T) {
	var gotAuth []string
	updated, _ := time.Parse(time.RFC3339, "2026-01-02T03:04:05Z")
	client := newFakeServer(t, "github", "tok123", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		if r.URL.Path != "/user/repos" {
			t.Errorf("应请求认证端点 /user/repos, got %s", r.URL.Path)
		}
		if page := r.URL.Query().Get("page"); page == "1" {
			writeJSON(t, w, []ghRepo{
				{Name: "cube", FullName: "heyuuu/cube", CloneUrl: "https://github.com/heyuuu/cube.git", DefaultBranch: "develop", UpdatedAt: updated},
				{Name: "b", FullName: "acme/b", CloneUrl: "https://github.com/acme/b.git", DefaultBranch: "main"},
			})
			return
		}
		writeJSON(t, w, []ghRepo{})
	})
	repos, err := client.ListAccountRepos(context.Background())
	if err != nil {
		t.Fatalf("ListAccountRepos 失败: %v", err)
	}
	if len(repos) != 2 || repos[0].Name != "cube" || repos[0].DefaultBranch != "develop" || repos[1].FullName != "acme/b" {
		t.Fatalf("仓库映射不符: %+v", repos)
	}
	if len(gotAuth) == 0 || gotAuth[0] != "Bearer tok123" {
		t.Fatalf("token 应走 Bearer header: %v", gotAuth)
	}
}

// TestGiteeAuthed gitee 走 /api/v5/user/repos，token 必须以 access_token query 参数发送
// （此前 query 认证发生在 URL 构造之后，token 只在 scheme 降级重试时碰巧发出——真实
// https 一次成功场景 token 从未生效，见 1044）。
func TestGiteeAuthed(t *testing.T) {
	var gotToken, gotAuthHeader string
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("access_token")
		gotAuthHeader = r.Header.Get("Authorization")
		if r.URL.Path != "/api/v5/user/repos" {
			t.Errorf("应请求 /api/v5/user/repos, got %s", r.URL.Path)
		}
		writeJSON(t, w, []giteeRepo{
			{Name: "go-lombok", FullName: "heyuuu/go-lombok", CloneUrl: "git@gitee.com:heyuuu/go-lombok.git", DefaultBranch: "master"},
		})
	}))
	t.Cleanup(ts.Close)
	u, _ := url.Parse(ts.URL)
	client, err := NewClient("gitee", u.Host, "gitee-tok", ts.Client())
	if err != nil {
		t.Fatalf("NewClient 失败: %v", err)
	}
	repos, err := client.ListAccountRepos(context.Background())
	if err != nil {
		t.Fatalf("ListAccountRepos 失败: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "go-lombok" {
		t.Fatalf("仓库映射不符: %+v", repos)
	}
	if gotToken != "gitee-tok" {
		t.Fatalf("https 首次请求必须带 access_token, got %q (Authorization=%q)", gotToken, gotAuthHeader)
	}
}

// TestGiteaAuthed gitea 走 /api/v1/user/repos，token 走 `Authorization: token` header。
func TestGiteaAuthed(t *testing.T) {
	var gotAuth, gotPath string
	client := newFakeServer(t, "gitea", "gt", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		writeJSON(t, w, []giteaRepo{})
	})
	if _, err := client.ListAccountRepos(context.Background()); err != nil {
		t.Fatalf("ListAccountRepos 失败: %v", err)
	}
	if gotPath != "/api/v1/user/repos" || gotAuth != "token gt" {
		t.Fatalf("端点/认证不符: path=%s auth=%q", gotPath, gotAuth)
	}
}

// TestUnauthorized 401/403 统一映射 ErrUnauthorized（gitee 认证在 query 上，401 也应正确冒泡）。
func TestUnauthorized(t *testing.T) {
	for _, kind := range []string{"github", "gitea", "gitee"} {
		client := newFakeServer(t, kind, "bad", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
		if _, err := client.ListAccountRepos(context.Background()); err != ErrUnauthorized {
			t.Errorf("%s: 401 应映射 ErrUnauthorized, got %v", kind, err)
		}
	}
}

// TestNewClientValidation 构造校验：token 必填、gitea 必须给 host、未知 kind 报错。
func TestNewClientValidation(t *testing.T) {
	if _, err := NewClient("github", "", "", nil); err != ErrUnauthorized {
		t.Fatalf("空 token 应返回 ErrUnauthorized, got %v", err)
	}
	if _, err := NewClient("gitea", "", "t", nil); err == nil {
		t.Fatal("gitea 无 host 应报错")
	}
	if _, err := NewClient("generic", "x.com", "t", nil); err == nil {
		t.Fatal("未知 kind 应报错")
	}
	if c, err := NewClient("github", "", "t", nil); err != nil || c == nil {
		t.Fatalf("github 空 host 应用默认域名: %v", err)
	}
}
