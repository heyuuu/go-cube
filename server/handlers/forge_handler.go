package handlers

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"cube/forge"
	"cube/project"
	"cube/util/iconkit"
	"cube/web"
)

// ForgeHandler forge 配置管理的 HTTP 出口（1040 forge / 1041 account+namespace）。
type ForgeHandler struct {
	forgeService   *forge.Service
	projectService *project.Service // 对账用：本地项目快照投影为 forge.LocalRepo
}

func NewForgeHandler(forgeService *forge.Service, projectService *project.Service) *ForgeHandler {
	return &ForgeHandler{forgeService: forgeService, projectService: projectService}
}

func (h *ForgeHandler) Register(api huma.API, mux *http.ServeMux) {
	web.ApiGet(api, "/api/forge/list", "获取 forge 列表", h.forgeList)
	web.ApiPost(api, "/api/forge/save", "新增或按 host 替换 forge", h.forgeSave)
	web.ApiPost(api, "/api/forge/delete", "按 host 删除 forge", h.forgeDelete)
	web.ApiPost(api, "/api/forge/reorder", "按 host 重排 forge 顺序", h.forgeReorder)

	web.ApiGet(api, "/api/forge/account/list", "获取 forge account 列表（token 打码）", h.accountList)
	web.ApiPost(api, "/api/forge/account/save", "新增或按 forgeHost+username 替换 account", h.accountSave)
	web.ApiPost(api, "/api/forge/account/delete", "按 forgeHost+username 删除 account", h.accountDelete)

	web.ApiGet(api, "/api/forge/namespace/list", "获取 forge namespace 列表", h.namespaceList)
	web.ApiPost(api, "/api/forge/namespace/save", "新增或按 forgeHost+path 替换 namespace", h.namespaceSave)
	web.ApiPost(api, "/api/forge/namespace/delete", "按 forgeHost+path 删除 namespace", h.namespaceDelete)
	web.ApiPost(api, "/api/forge/namespace/fetch", "拉取 namespace 下远端仓库列表", h.namespaceFetch)
	web.ApiPost(api, "/api/forge/namespace/detect", "探测 namespace 是个人空间还是组织空间", h.namespaceDetect)
	web.ApiGet(api, "/api/forge/namespace/reconcile", "对账 namespace（远端缓存 vs 本地项目）", h.namespaceReconcile)
}

func (h *ForgeHandler) forgeList(_ struct{}) (web.ListResult[forge.Forge], error) {
	return listResult(h.forgeService.Forges()), nil
}

// ForgeSaveInput forge/save 接口入参（host 归一化后是唯一键）。
type ForgeSaveInput struct {
	Body struct {
		Host string   `json:"host" doc:"域名（可带端口），如 github.com"`
		Kind string   `json:"kind" doc:"API 方言：github / gitea / gitee / generic"`
		Icon *IconDTO `json:"icon,omitempty" doc:"平台图标（可选：lucide 图名或 base64 PNG）"`
	}
}

func (h *ForgeHandler) forgeSave(input ForgeSaveInput) (map[string]any, error) {
	f := forge.Forge{Host: input.Body.Host, Kind: input.Body.Kind}
	if input.Body.Icon != nil {
		f.Icon = &iconkit.Icon{Type: input.Body.Icon.Type, Value: input.Body.Icon.Value}
	}
	if err := h.forgeService.SaveForge(f); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// ForgeDeleteInput forge/delete 接口入参。
type ForgeDeleteInput struct {
	Body struct {
		Host string `json:"host" doc:"规则唯一键"`
	}
}

func (h *ForgeHandler) forgeDelete(input ForgeDeleteInput) (map[string]any, error) {
	if err := h.forgeService.DeleteForge(input.Body.Host); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// ForgeReorderInput forge/reorder 接口入参（整表按目标顺序提交 host 名单）。
type ForgeReorderInput struct {
	Body struct {
		Hosts []string `json:"hosts" doc:"按目标顺序排列的 host 名单"`
	}
}

func (h *ForgeHandler) forgeReorder(input ForgeReorderInput) (map[string]any, error) {
	if err := h.forgeService.ReorderForges(input.Body.Hosts); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// --- account（token 只以打码形态出 API；save 提交掩码值 = 未修改） ---

func (h *ForgeHandler) accountList(_ struct{}) (web.ListResult[forge.Account], error) {
	accounts := h.forgeService.Accounts()
	for i := range accounts {
		if accounts[i].Token != "" {
			accounts[i].Token = forge.TokenMasked
		}
	}
	return listResult(accounts), nil
}

// AccountSaveInput account/save 接口入参。
type AccountSaveInput struct {
	Body struct {
		ForgeHost string `json:"forgeHost" doc:"所属 forge host（唯一键之一）"`
		Username  string `json:"username" doc:"平台用户名（唯一键之一）"`
		Token     string `json:"token,omitempty" doc:"API token；提交掩码值视为未修改"`
	}
}

func (h *ForgeHandler) accountSave(input AccountSaveInput) (map[string]any, error) {
	b := input.Body
	if err := h.forgeService.SaveAccount(forge.Account{ForgeHost: b.ForgeHost, Username: b.Username, Token: b.Token}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// AccountDeleteInput account/delete 接口入参。
type AccountDeleteInput struct {
	Body struct {
		ForgeHost string `json:"forgeHost"`
		Username  string `json:"username"`
	}
}

func (h *ForgeHandler) accountDelete(input AccountDeleteInput) (map[string]any, error) {
	if err := h.forgeService.DeleteAccount(input.Body.ForgeHost, input.Body.Username); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// --- namespace ---

func (h *ForgeHandler) namespaceList(_ struct{}) (web.ListResult[forge.Namespace], error) {
	return listResult(h.forgeService.Namespaces()), nil
}

// NamespaceSaveInput namespace/save 接口入参。
type NamespaceSaveInput struct {
	Body struct {
		ForgeHost       string `json:"forgeHost" doc:"所属 forge host（唯一键之一）"`
		Path            string `json:"path" doc:"命名空间路径，如 heyuuu（唯一键之一）"`
		Type            string `json:"type" doc:"personal / org"`
		AccountUsername string `json:"accountUsername,omitempty" doc:"拉取私有库用的 account（可选）"`
	}
}

func (h *ForgeHandler) namespaceSave(input NamespaceSaveInput) (map[string]any, error) {
	b := input.Body
	ns := forge.Namespace{ForgeHost: b.ForgeHost, Path: b.Path, Type: forge.NamespaceType(b.Type), AccountUsername: b.AccountUsername}
	if err := h.forgeService.SaveNamespace(ns); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// NamespaceDeleteInput namespace/delete 接口入参。
type NamespaceDeleteInput struct {
	Body struct {
		ForgeHost string `json:"forgeHost"`
		Path      string `json:"path"`
	}
}

func (h *ForgeHandler) namespaceDelete(input NamespaceDeleteInput) (map[string]any, error) {
	if err := h.forgeService.DeleteNamespace(input.Body.ForgeHost, input.Body.Path); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// NamespaceFetchInput namespace/fetch 接口入参（出站 API 调用，同步执行）。
type NamespaceFetchInput struct {
	Body struct {
		ForgeHost string `json:"forgeHost"`
		Path      string `json:"path"`
		Force     bool   `json:"force,omitempty" doc:"跳过缓存强制重拉"`
	}
}

func (h *ForgeHandler) namespaceFetch(input NamespaceFetchInput) (map[string]any, error) {
	b := input.Body
	repos, err := h.forgeService.FetchNamespace(b.ForgeHost, b.Path, b.Force)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "count": len(repos)}, nil
}

// NamespaceDetectInput namespace/detect 接口入参。
type NamespaceDetectInput struct {
	Body struct {
		ForgeHost string `json:"forgeHost"`
		Path      string `json:"path"`
	}
}

func (h *ForgeHandler) namespaceDetect(input NamespaceDetectInput) (map[string]any, error) {
	b := input.Body
	nsType, err := h.forgeService.DetectNamespaceType(b.ForgeHost, b.Path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "type": nsType}, nil
}

// NamespaceReconcileInput namespace/reconcile 接口入参（query）。
type NamespaceReconcileInput struct {
	ForgeHost string `query:"forgeHost" doc:"所属 forge host"`
	Path      string `query:"path" doc:"命名空间路径"`
}

func (h *ForgeHandler) namespaceReconcile(input NamespaceReconcileInput) (*forge.ReconcileResult, error) {
	var locals []forge.LocalRepo
	for _, p := range h.projectService.Projects() {
		info := forge.LocalRepo{Name: p.Name(), Path: p.Path()}
		if gi, ok := h.projectService.GitInfo(p.Path()); ok {
			info.RepoUrl = gi.RepoUrl
			info.Dirty = gi.Dirty
			info.Ahead = gi.Ahead
			info.Behind = gi.Behind
		}
		locals = append(locals, info)
	}
	return h.forgeService.ReconcileNamespace(input.ForgeHost, input.Path, locals)
}
