package handlers

import (
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"cube/project"
	"cube/project/gitcache"
	"cube/util/iconkit"
	"cube/util/slicekit"
	"cube/web"
)

// --- dto ---
type ProjectDTO struct {
	Name    string          `json:"name"`
	Group   string          `json:"group"`
	Path    string          `json:"path"`
	Tags    []string        `json:"tags"`
	GitInfo *gitcache.Entry `json:"gitInfo"` // git 状态快照，可能为 nil（未采集）
}

// ProjectListResult 列表接口返回结构：含项目列表 + 两类刷新时间。
type ProjectListResult struct {
	List          []*ProjectDTO `json:"list"`
	ScanUpdatedAt time.Time     `json:"scanUpdatedAt"` // 项目列表最近一次重扫完成时间（零值=未刷新过）
	GitUpdatedAt  time.Time     `json:"gitUpdatedAt"`  // git 缓存最近一次落盘时间（零值=无缓存）
}

// --- handler ---

type ProjectHandler struct {
	projectService *project.Service
}

func NewProjectHandler(projectService *project.Service) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService,
	}
}

func (h *ProjectHandler) Register(api huma.API, mux *http.ServeMux) {
	web.ApiGet(api, "/api/project/list", "获取项目列表", h.projectList)
	web.ApiGet(api, "/api/project/info", "获取项目详情", h.projectInfo)
	web.ApiGet(api, "/api/project/scan-rules", "获取扫描规则", h.scanRules)
	web.ApiGet(api, "/api/project/clone-rules", "获取 clone 规则", h.cloneRules)
	web.ApiPost(api, "/api/project/scan-rule/save", "新增或按 path 替换扫描规则", h.scanRuleSave)
	web.ApiPost(api, "/api/project/scan-rule/delete", "按 path 删除扫描规则", h.scanRuleDelete)
	web.ApiPost(api, "/api/project/scan-rule/reorder", "按 path 重排扫描规则顺序", h.scanRuleReorder)
	web.ApiPost(api, "/api/project/clone-rule/save", "新增或按 host+prefix 替换 clone 规则", h.cloneRuleSave)
	web.ApiPost(api, "/api/project/clone-rule/delete", "按 host+prefix 删除 clone 规则", h.cloneRuleDelete)
	web.ApiPost(api, "/api/project/clone-rule/reorder", "按键重排 clone 规则顺序", h.cloneRuleReorder)
}

func (h *ProjectHandler) projectList(_ struct{}) (ProjectListResult, error) {
	projects := h.projectService.Projects()
	list := slicekit.Map(projects, h.toProjectDTO)
	return ProjectListResult{
		List:          list,
		ScanUpdatedAt: h.projectService.ScanUpdatedAt(),
		GitUpdatedAt:  h.projectService.GitUpdatedAt(),
	}, nil
}

// ProjectInfoResult 详情接口返回结构：含项目 DTO + 两类刷新时间。
type ProjectInfoResult struct {
	Project       *ProjectDTO `json:"project"`
	ScanUpdatedAt time.Time   `json:"scanUpdatedAt"`
	GitUpdatedAt  time.Time   `json:"gitUpdatedAt"`
}

func (h *ProjectHandler) projectInfo(input struct {
	Name string `query:"name" required:"true"`
}) (ProjectInfoResult, error) {
	proj := h.projectService.FindByName(input.Name)
	return ProjectInfoResult{
		Project:       h.toProjectDTO(proj),
		ScanUpdatedAt: h.projectService.ScanUpdatedAt(),
		GitUpdatedAt:  h.projectService.GitUpdatedAt(),
	}, nil
}

func (h *ProjectHandler) toProjectDTO(entity *project.Project) *ProjectDTO {
	if entity == nil {
		return nil
	}

	info, _ := h.projectService.GitInfo(entity.Path())
	return &ProjectDTO{
		Name:    entity.Name(),
		Group:   entity.Group(),
		Path:    entity.Path(),
		Tags:    entity.Tags(),
		GitInfo: info,
	}
}

func (h *ProjectHandler) scanRules(_ struct{}) (web.ListResult[project.ScanRule], error) {
	rules := h.projectService.ScanRules()
	return listResult(rules), nil
}

func (h *ProjectHandler) cloneRules(_ struct{}) (web.ListResult[project.CloneRule], error) {
	rules := h.projectService.CloneRules()
	return listResult(rules), nil
}

// --- 规则写侧（校验在 Service 写侧，坏数据返回中文错误、不落文件） ---

// ScanRuleSaveInput scan-rule/save 接口入参（字段与 project.ScanRule 对齐，path 是唯一键）。
type ScanRuleSaveInput struct {
	Body struct {
		Group    string   `json:"group" doc:"扫描出的项目组名"`
		Path     string   `json:"path" doc:"扫描根目录（绝对路径或 ~/ 前缀，规则唯一键）"`
		MaxDepth int      `json:"maxDepth" doc:"扫描最大深度"`
		Icon     *IconDTO `json:"icon,omitempty" doc:"组图标（可选：lucide 图名或 base64 PNG）"`
	}
}

func (h *ProjectHandler) scanRuleSave(input ScanRuleSaveInput) (map[string]any, error) {
	rule := project.ScanRule{Group: input.Body.Group, Path: input.Body.Path, MaxDepth: input.Body.MaxDepth}
	if input.Body.Icon != nil {
		rule.Icon = &iconkit.Icon{Type: input.Body.Icon.Type, Value: input.Body.Icon.Value}
	}
	if err := h.projectService.SaveScanRule(rule); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// ScanRuleDeleteInput scan-rule/delete 接口入参。
type ScanRuleDeleteInput struct {
	Body struct {
		Path string `json:"path" doc:"规则唯一键"`
	}
}

func (h *ProjectHandler) scanRuleDelete(input ScanRuleDeleteInput) (map[string]any, error) {
	if err := h.projectService.DeleteScanRule(input.Body.Path); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// ScanRuleReorderInput scan-rule/reorder 接口入参（整表按目标顺序提交路径名单）。
type ScanRuleReorderInput struct {
	Body struct {
		Paths []string `json:"paths" doc:"按目标顺序排列的规则路径名单"`
	}
}

func (h *ProjectHandler) scanRuleReorder(input ScanRuleReorderInput) (map[string]any, error) {
	if err := h.projectService.ReorderScanRules(input.Body.Paths); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// CloneRuleSaveInput clone-rule/save 接口入参（host+prefix 是唯一键）。
type CloneRuleSaveInput struct {
	Body struct {
		RepoHost   string `json:"repoHost" doc:"源域名，无协议，如 github.com"`
		RepoPrefix string `json:"repoPrefix,omitempty" doc:"uri 前缀，须以 / 开头或为空，如 /heyuuu"`
		LocalPath  string `json:"localPath" doc:"对应本地目录（绝对路径或 ~/ 前缀）"`
	}
}

func (h *ProjectHandler) cloneRuleSave(input CloneRuleSaveInput) (map[string]any, error) {
	rule := project.CloneRule{
		RepoHost:   input.Body.RepoHost,
		RepoPrefix: input.Body.RepoPrefix,
		LocalPath:  input.Body.LocalPath,
	}
	if err := h.projectService.SaveCloneRule(rule); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// CloneRuleDeleteInput clone-rule/delete 接口入参。
type CloneRuleDeleteInput struct {
	Body struct {
		RepoHost   string `json:"repoHost" doc:"规则键：源域名"`
		RepoPrefix string `json:"repoPrefix" doc:"规则键：uri 前缀"`
	}
}

func (h *ProjectHandler) cloneRuleDelete(input CloneRuleDeleteInput) (map[string]any, error) {
	key := project.CloneRuleKey{RepoHost: input.Body.RepoHost, RepoPrefix: input.Body.RepoPrefix}
	if err := h.projectService.DeleteCloneRule(key); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// CloneRuleReorderInput clone-rule/reorder 接口入参（整表按目标顺序提交键名单）。
type CloneRuleReorderInput struct {
	Body struct {
		Rules []project.CloneRuleKey `json:"rules" doc:"按目标顺序排列的 host+prefix 名单"`
	}
}

func (h *ProjectHandler) cloneRuleReorder(input CloneRuleReorderInput) (map[string]any, error) {
	if err := h.projectService.ReorderCloneRules(input.Body.Rules); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}
