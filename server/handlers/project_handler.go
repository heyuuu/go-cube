package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"cube/opener"
	"cube/project"
	"cube/project/cubefile"
	"cube/project/projcache"
	"cube/project/workspace"
	"cube/usage"
	"cube/util/iconkit"
	"cube/util/slicekit"
	"cube/web"
)

// --- dto ---
type ProjectDTO struct {
	Name       string           `json:"name"`
	Group      string           `json:"group"`
	Path       string           `json:"path"`
	Tags       []string         `json:"tags"`
	GitInfo    *projcache.Entry `json:"gitInfo"`              // git 状态快照，可能为 nil（未采集）
	LastUsedAt *time.Time       `json:"lastUsedAt,omitempty"` // 最近使用时间（置顶排序信号，未用过为 nil）
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
	openerService  *opener.Service
	usageService   *usage.Service
}

func NewProjectHandler(projectService *project.Service, openerService *opener.Service, usageService *usage.Service) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService,
		openerService:  openerService,
		usageService:   usageService,
	}
}

func (h *ProjectHandler) Register(api huma.API, mux *http.ServeMux) {
	web.ApiGet(api, "/api/project/list", "获取项目列表", h.projectList)
	web.ApiGet(api, "/api/project/info", "获取项目详情", h.projectInfo)
	web.ApiPost(api, "/api/project/open", "用指定 opener 打开已收录项目（记 usage；目标目录选择为 1030/1032 预留）", h.projectOpen)
	web.ApiGet(api, "/api/project/workspace/get", "获取项目 workspace 状态（生效清单 / 显式声明 / 探测候选）", h.workspaceGet)
	web.ApiPost(api, "/api/project/workspace/save", "保存显式 workspaces 声明（.cube/cube.json）并即时重采集", h.workspaceSave)
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
	// 返回原始扫描序 + lastUsedAt；排序是视图偏好，由前端做（CLI/alfred 出口用 project.SortByRecentUsage）
	projects := h.projectService.Projects()
	latest := h.usageService.LatestByProject()
	list := slicekit.Map(projects, func(p *project.Project) *ProjectDTO {
		dto := h.toProjectDTO(p)
		if t, ok := latest[p.Path()]; ok {
			tt := t
			dto.LastUsedAt = &tt
		}
		return dto
	})
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
	Path string `query:"path" required:"true"`
}) (ProjectInfoResult, error) {
	proj := h.projectService.FindByPath(input.Path)
	return ProjectInfoResult{
		Project:       h.toProjectDTO(proj),
		ScanUpdatedAt: h.projectService.ScanUpdatedAt(),
		GitUpdatedAt:  h.projectService.GitUpdatedAt(),
	}, nil
}

// ProjectOpenInput open 接口入参。huma 约定：请求体字段须挂在名为 Body 的子结构上。
// path 是项目绝对路径（项目唯一标识口径）；dir 可选，为目标目录（worktree，1032），
// 缺省/等于项目根 = 打开根目录。
type ProjectOpenInput struct {
	Body struct {
		Path   string `json:"path" doc:"项目绝对路径"`
		Dir    string `json:"dir,omitempty" doc:"目标目录绝对路径（worktree；缺省打开项目根）"`
		Opener string `json:"opener" doc:"opener 名称"`
	}
}

// projectOpen 用指定 opener 打开已收录项目的指定目标目录，成功后记 usage（best-effort：失败不影响打开结果）。
// 区别于 opener/open（打开任意路径，不产生使用信号）。
func (h *ProjectHandler) projectOpen(input ProjectOpenInput) (map[string]any, error) {
	proj := h.projectService.FindByPath(input.Body.Path)
	if proj == nil {
		return nil, errors.New("未找到指定项目: " + input.Body.Path)
	}

	// 目标目录校验：只允许项目自身的目标（根目录 + 快照内 worktree），不开放任意路径
	target := proj.Path()
	if input.Body.Dir != "" && input.Body.Dir != proj.Path() {
		if !slices.ContainsFunc(h.projectService.OpenTargets(proj.Path()), func(t project.OpenTarget) bool {
			return t.Path == input.Body.Dir
		}) {
			// OpenTargets 已按存在性过滤：命中这里的目标目录多半是快照过期残留
			if _, err := os.Stat(input.Body.Dir); err != nil {
				return nil, errors.New("目标目录已不存在（快照过期，等下次采集刷新后重试）: " + input.Body.Dir)
			}
			return nil, errors.New("目标目录不属于该项目: " + input.Body.Dir)
		}
		target = input.Body.Dir
	}

	o := h.openerService.FindByName(input.Body.Opener)
	if o == nil {
		return nil, errors.New("未找到指定 opener: " + input.Body.Opener)
	}

	if err := o.Open(opener.RoleOpenDir, target); err != nil {
		return nil, fmt.Errorf("打开失败: %w", err)
	}

	// project 恒记主项目路径；dir 直接传目标目录（等于根时由 RecordOpen 归一为空）
	if err := h.usageService.RecordOpen(proj.Path(), o.Name(), target); err != nil {
		slog.Warn("记录 usage 失败", "err", err)
	}
	return map[string]any{"ok": true}, nil
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

// --- workspace 声明（1030）---

// WorkspaceMemberDTO workspace 成员（显示名 + 相对所属根路径）。
type WorkspaceMemberDTO struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// WorkspaceStateResult workspace/get 返回：编辑视图所需的三块信息。
// Effective 读 projcache 快照（读路径不读声明文件、不探测）；Declared/Detected 是
// 编辑场景的一次性现算（编辑场景可以严格，与「读路径纯快照」纪律不冲突）。
type WorkspaceStateResult struct {
	Effective   []WorkspaceMemberDTO `json:"effective"`   // 当前生效清单（显式声明或探测结果）
	DeclaredSet bool                 `json:"declaredSet"` // cube.json 是否有显式 workspaces 字段
	Declared    []WorkspaceMemberDTO `json:"declared"`    // 显式声明原文（declaredSet=false 时为空）
	ScanRule    string               `json:"scanRule"`    // cube.json 的 workspaceScanRule（空=用默认规则）
	Detected    []WorkspaceMemberDTO `json:"detected"`    // 探测候选（「导入为候选」入口用；未命中为空）
}

func (h *ProjectHandler) workspaceGet(input struct {
	Path string `query:"path" required:"true"`
}) (WorkspaceStateResult, error) {
	proj := h.projectService.FindByPath(input.Path)
	if proj == nil {
		return WorkspaceStateResult{}, errors.New("未找到指定项目: " + input.Path)
	}
	root := proj.Path()
	result := WorkspaceStateResult{Effective: []WorkspaceMemberDTO{}, Declared: []WorkspaceMemberDTO{}, Detected: []WorkspaceMemberDTO{}}
	if info, ok := h.projectService.GitInfo(root); ok && info != nil {
		result.Effective = slicekit.Map(info.Workspaces, func(w workspace.Workspace) WorkspaceMemberDTO {
			return WorkspaceMemberDTO(w)
		})
	}
	if cf, ok := cubefile.Load(root); ok {
		result.DeclaredSet = cf.WorkspacesSet
		result.Declared = slicekit.Map(cf.Workspaces, func(d cubefile.Declared) WorkspaceMemberDTO {
			return WorkspaceMemberDTO(d)
		})
		result.ScanRule = cf.WorkspaceScanRule
	}
	// 探测候选独立于 cube.json 是否存在（无声明项目的「导入为候选」入口同样要工作）
	result.Detected = slicekit.Map(workspace.Detect(root, workspace.EffectiveScanRule(root)), func(w workspace.Workspace) WorkspaceMemberDTO {
		return WorkspaceMemberDTO(w)
	})
	return result, nil
}

// WorkspaceSaveInput workspace/save 接口入参。workspaces 至少一条（清除声明直接删 .cube/cube.json，不开 API）。
type WorkspaceSaveInput struct {
	Body struct {
		Path       string               `json:"path" doc:"项目绝对路径"`
		Workspaces []WorkspaceMemberDTO `json:"workspaces" doc:"显式声明的成员清单（name + 相对项目根路径）"`
	}
}

// workspaceSave 写侧校验严格（坏条目整体拒绝，静默跳过是采集侧的降级语义），
// 成功后定向重采集让打开目标即刻反映。
func (h *ProjectHandler) workspaceSave(input WorkspaceSaveInput) (map[string]any, error) {
	proj := h.projectService.FindByPath(input.Body.Path)
	if proj == nil {
		return nil, errors.New("未找到指定项目: " + input.Body.Path)
	}
	if len(input.Body.Workspaces) == 0 {
		return nil, errors.New("workspaces 不能为空（清除声明请删除项目内 .cube/cube.json）")
	}
	root := proj.Path()
	declared := slicekit.Map(input.Body.Workspaces, func(m WorkspaceMemberDTO) cubefile.Declared {
		return cubefile.Declared(m)
	})
	validated := workspace.ValidateDeclared(root, declared)
	if len(validated) != len(declared) {
		return nil, errors.New("存在无效条目（path 须相对项目根、目录存在且不逃逸），请修正后重试")
	}
	if err := workspace.Save(root, validated); err != nil {
		return nil, err
	}
	if err := h.projectService.RefreshGitInfo(root); err != nil {
		slog.Warn("workspace 保存后刷新快照失败（等下次采集自愈）", "err", err)
	}
	return map[string]any{"ok": true}, nil
}
