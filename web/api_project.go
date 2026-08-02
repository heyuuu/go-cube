package web

import (
	"errors"
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/heyuuu/cube/opener"
	"github.com/heyuuu/cube/project"
	"github.com/heyuuu/cube/project/gitcache"
	"github.com/heyuuu/cube/util/slicekit"
)

// --- dto ---

type ProjectDTO struct {
	Name    string          `json:"name"`
	Group   string          `json:"group"`
	Path    string          `json:"path"`
	RepoUrl string          `json:"repoUrl"`
	Tags    []string        `json:"tags"`
	GitInfo *gitcache.Entry `json:"gitInfo"` // git 状态快照，可能为 nil（未采集）
}

func toProjectDTO(entity *project.Project) *ProjectDTO {
	if entity == nil {
		return nil
	}

	return &ProjectDTO{
		Name:    entity.Name(),
		Group:   entity.Group(),
		Path:    entity.Path(),
		RepoUrl: entity.RepoUrl(),
		Tags:    entity.Tags(),
		GitInfo: entity.GitInfo(),
	}
}

// ProjectListResult 列表接口返回结构：含项目列表 + git 缓存整体刷新时间。
type ProjectListResult struct {
	List            []*ProjectDTO `json:"list"`
	GitCacheUpdated time.Time     `json:"gitCacheUpdated"` // git 缓存最近一次落盘时间（零值=无缓存）
}

// --- handler ---

type ProjectHandler struct {
	service       *project.Service
	openerService *opener.Service
}

func NewProjectHandler(service *project.Service, openerService *opener.Service) *ProjectHandler {
	return &ProjectHandler{
		service:       service,
		openerService: openerService,
	}
}

func (h *ProjectHandler) Register(api huma.API) {
	apiGet(api, "/api/project/list", "获取项目列表", h.projectList)
	apiGet(api, "/api/project/info", "获取项目详情", h.projectInfo)
	apiGet(api, "/api/project/scan-rules", "获取扫描规则", h.scanRules)
	apiGet(api, "/api/project/clone-rules", "获取 clone 规则", h.cloneRules)
	apiPost(api, "/api/project/open", "用指定 opener 打开项目", h.projectOpen)
}

func (h *ProjectHandler) projectList(_ struct{}) (ProjectListResult, error) {
	projects := h.service.Projects()
	list := slicekit.Map(projects, toProjectDTO)
	return ProjectListResult{
		List:            list,
		GitCacheUpdated: h.service.GitCacheUpdatedAt(),
	}, nil
}

func (h *ProjectHandler) projectInfo(input struct {
	Name string `json:"name"`
}) (result *ProjectDTO, err error) {
	proj := h.service.FindByName(input.Name)
	return toProjectDTO(proj), nil
}

func (h *ProjectHandler) scanRules(_ struct{}) (ListResult[project.ScanRule], error) {
	rules := h.service.ScanRules()
	return listResult(rules), nil
}

func (h *ProjectHandler) cloneRules(_ struct{}) (ListResult[project.CloneRule], error) {
	rules := h.service.CloneRules()
	return listResult(rules), nil
}

// ProjectOpenInput open 接口入参。huma 约定：请求体字段须挂在名为 Body 的子结构上。
type ProjectOpenInput struct {
	Body struct {
		Path string `json:"path" doc:"项目绝对路径"`
		App  string `json:"app" doc:"opener 名称（finder / vscode / idea ...）"`
	}
}

func (h *ProjectHandler) projectOpen(input ProjectOpenInput) (map[string]any, error) {
	// 校验 opener
	openApp := h.openerService.FindByName(input.Body.App)
	if openApp == nil {
		return nil, fmt.Errorf("未找到指定 app: %s", input.Body.App)
	}

	// 校验项目
	proj := h.service.FindByPath(input.Body.Path)
	if proj == nil {
		return nil, errors.New("未找到指定项目: " + input.Body.Path)
	}

	// 打开（opener 是 fire-and-forget，启动子进程后立即返回）
	if err := openApp.Open(proj.Path()); err != nil {
		return nil, fmt.Errorf("打开失败: %w", err)
	}

	return map[string]any{"ok": true}, nil
}
