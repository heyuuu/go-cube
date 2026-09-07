package handlers

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"cube/forge"
	"cube/project"
	"cube/util/iconkit"
	"cube/web"
)

// ForgeHandler forge 配置管理的 HTTP 出口（1040 forge / 1041 account / 1042 forge 页）。
// 1044 起 namespace 模型移除，account 是 forge 下唯一的关联配置。
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
	web.ApiPost(api, "/api/forge/account/reorder", "按键（host/username）重排 account 顺序", h.accountReorder)
	web.ApiPost(api, "/api/forge/account/fetch", "拉取 account 名下全部远端仓库", h.accountFetch)

	web.ApiGet(api, "/api/forge/overview", "forge 页聚合：全部 account 对账行 + 拉取元信息（只读缓存不外呼）", h.forgeOverview)
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

// AccountFetchInput account/fetch 接口入参（出站 API 调用，同步执行）。
type AccountFetchInput struct {
	Body struct {
		ForgeHost string `json:"forgeHost"`
		Username  string `json:"username"`
		Force     bool   `json:"force,omitempty" doc:"跳过缓存强制重拉"`
	}
}

func (h *ForgeHandler) accountFetch(input AccountFetchInput) (map[string]any, error) {
	b := input.Body
	repos, err := h.forgeService.FetchAccount(b.ForgeHost, b.Username, b.Force)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "count": len(repos)}, nil
}

// AccountReorderInput account/reorder 接口入参（整表按目标顺序提交键名单）。
type AccountReorderInput struct {
	Body struct {
		Keys []string `json:"keys" doc:"按目标顺序排列的 account 键名单（host/username）"`
	}
}

func (h *ForgeHandler) accountReorder(input AccountReorderInput) (map[string]any, error) {
	if err := h.forgeService.ReorderAccounts(input.Body.Keys); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// forgeOverview forge 页聚合数据源（只读拉取缓存与本地快照，不触发外呼）。
func (h *ForgeHandler) forgeOverview(_ struct{}) (*forge.Overview, error) {
	return h.forgeService.Overview(h.localRepos()), nil
}

// localRepos 把本地项目快照投影为对账用的 LocalRepo 列表。
func (h *ForgeHandler) localRepos() []forge.LocalRepo {
	locals := make([]forge.LocalRepo, 0)
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
	return locals
}
