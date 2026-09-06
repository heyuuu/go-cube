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

// TestGithubListPaged 分页拉全量 + token header + 字段映射。
func TestGithubListPaged(t *testing.T) {
	var gotAuth []string
	updated, _ := time.Parse(time.RFC3339, "2026-01-02T03:04:05Z")
	client := newFakeServer(t, "github", "tok123", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		page := r.URL.Query().Get("page")
		if page == "1" {
			writeJSON(t, w, []ghRepo{
				{Name: "cube", FullName: "heyuuu/cube", CloneUrl: "https://github.com/heyuuu/cube.git", DefaultBranch: "develop", UpdatedAt: updated},
				{Name: "b", FullName: "heyuuu/b", CloneUrl: "https://github.com/heyuuu/b.git", DefaultBranch: "main"},
			})
			return
		}
		writeJSON(t, w, []ghRepo{})
	})
	repos, err := client.ListNamespaceRepos(context.Background(), NamespacePersonal, "heyuuu")
	if err != nil {
		t.Fatalf("ListNamespaceRepos 失败: %v", err)
	}
	if len(repos) != 2 || repos[0].Name != "cube" || repos[0].DefaultBranch != "develop" {
		t.Fatalf("仓库映射不符: %+v", repos)
	}
	if len(gotAuth) == 0 || gotAuth[0] != "Bearer tok123" {
		t.Fatalf("token 应走 Bearer header: %v", gotAuth)
	}
}

// TestDetect 命中 /users → personal；/users 404 + /orgs 200 → org；双 404 → ErrNamespaceNotFound。
func TestDetect(t *testing.T) {
	userOnly := newFakeServer(t, "gitea", "", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/users/foo":
			writeJSON(t, w, map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	if got, err := userOnly.DetectNamespace(context.Background(), "foo"); err != nil || got != NamespacePersonal {
		t.Fatalf("userOnly 探测应为 personal, got %v err %v", got, err)
	}

	orgOnly := newFakeServer(t, "gitea", "", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/orgs/myorg":
			writeJSON(t, w, map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	if got, err := orgOnly.DetectNamespace(context.Background(), "myorg"); err != nil || got != NamespaceOrg {
		t.Fatalf("orgOnly 探测应为 org, got %v err %v", got, err)
	}

	none := newFakeServer(t, "gitea", "", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if _, err := none.DetectNamespace(context.Background(), "ghost"); err != ErrNamespaceNotFound {
		t.Fatalf("双 404 应返回 ErrNamespaceNotFound, got %v", err)
	}
}

// TestGiteeAuthQuery gitee 的 token 走 access_token query 参数。
func TestGiteeAuthQuery(t *testing.T) {
	var gotToken string
	client := newFakeServer(t, "gitee", "gitee-tok", func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("access_token")
		writeJSON(t, w, []giteeRepo{})
	})
	if _, err := client.ListNamespaceRepos(context.Background(), NamespaceOrg, "acme"); err != nil {
		t.Fatalf("ListNamespaceRepos 失败: %v", err)
	}
	if gotToken != "gitee-tok" {
		t.Fatalf("gitee token 应走 access_token query 参数, got %q", gotToken)
	}
}

// TestNewClientValidation 构造校验：gitea 必须给 host、未知 kind 报错。
func TestNewClientValidation(t *testing.T) {
	if _, err := NewClient("gitea", "", "", nil); err == nil {
		t.Fatal("gitea 无 host 应报错")
	}
	if _, err := NewClient("generic", "x.com", "", nil); err == nil {
		t.Fatal("未知 kind 应报错")
	}
	if c, err := NewClient("github", "", "", nil); err != nil || c == nil {
		t.Fatalf("github 空 host 应用默认域名: %v", err)
	}
}
