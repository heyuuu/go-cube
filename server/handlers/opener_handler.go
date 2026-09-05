package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"

	"cube/opener"
	"cube/util/slicekit"
	"cube/web"
)

// --- dto ---

type OpenerDTO struct {
	Name    string            `json:"name"`
	Title   string            `json:"title"`   // 展示文案（如「打开所在目录」），缺省由 name 生成
	Summary string            `json:"summary"` // 动作串展示串（"role:动作串" 拼接）
	Actions map[string]string `json:"actions"` // role → 动作串原文（`<kind>:<模板>`，编辑表单回显用）；键集合即声明的 roles
	Icon    IconDTO           `json:"icon"`    // 恒有值（未配置时后端按主 role 填默认 lucide 图）
}

// IconDTO icon 声明的 API 形态（opener / scanRule 等共用，语义见 util/iconkit）。
type IconDTO struct {
	Type  string `json:"type"`  // lucide | image
	Value string `json:"value"` // lucide 图名 或 base64 PNG
}

func toOpenerDTO(entity opener.Opener) *OpenerDTO {
	if entity == nil {
		return nil
	}

	actions := entity.Actions()
	dtoActions := make(map[string]string, len(actions))
	for r, raw := range actions {
		dtoActions[string(r)] = raw
	}
	return &OpenerDTO{
		Name:    entity.Name(),
		Title:   entity.Title(),
		Summary: entity.Summary(),
		Actions: dtoActions,
		Icon:    IconDTO{Type: entity.Icon().Type, Value: entity.Icon().Value},
	}
}

// --- handler ---

type OpenerHandler struct {
	service *opener.Service
}

func NewOpenerHandler(service *opener.Service) *OpenerHandler {
	return &OpenerHandler{
		service: service,
	}
}

func (h *OpenerHandler) Register(api huma.API, mux *http.ServeMux) {
	web.ApiGet(api, "/api/opener/list", "获取 opener 列表", h.openerList)
	web.ApiGet(api, "/api/opener/intents", "获取打开意图清单（intent → 默认 opener + 候选）", h.openerIntents)
	web.ApiGet(api, "/api/opener/info", "获取 opener 详情", h.openerInfo)
	web.ApiPost(api, "/api/opener/open", "用指定 opener 打开任意文件或目录", h.openerOpen)
	web.ApiPost(api, "/api/opener/diff-open", "用指定 opener 对比两个路径（diff-dir/diff-file）", h.openerDiffOpen)
	web.ApiPost(api, "/api/opener/save", "新增或更新 opener（按 name 替换）", h.openerSave)
	web.ApiPost(api, "/api/opener/delete", "按名删除 opener", h.openerDelete)
	web.ApiPost(api, "/api/opener/reorder", "按名重排 opener 顺序", h.openerReorder)
	web.ApiPost(api, "/api/opener/intent-default/save", "设置某 intent 的默认 opener", h.intentDefaultSave)
	web.ApiPost(api, "/api/opener/intent-default/delete", "清除某 intent 的默认 opener", h.intentDefaultDelete)
}

func (h *OpenerHandler) openerList(_ struct{}) (web.ListResult[*OpenerDTO], error) {
	openers := h.service.AllOpeners()
	list := slicekit.Map(openers, toOpenerDTO)
	return listResult(list), nil
}

// OpenerIntentDTO intents 接口输出条目。
type OpenerIntentDTO struct {
	Intent        string   `json:"intent"`
	DefaultOpener string   `json:"defaultOpener,omitempty"`
	Openers       []string `json:"openers"` // 候选清单（缺省 = 声明了对应 role 的全部 opener，读侧合成）
}

func (h *OpenerHandler) openerIntents(_ struct{}) (web.ListResult[*OpenerIntentDTO], error) {
	list := slicekit.Map(h.service.Intents(), func(i opener.IntentInfo) *OpenerIntentDTO {
		return &OpenerIntentDTO{
			Intent:        string(i.Intent),
			DefaultOpener: i.DefaultOpener,
			Openers:       i.Openers,
		}
	})
	return listResult(list), nil
}

func (h *OpenerHandler) openerInfo(input struct {
	Name string `query:"name" required:"true"`
}) (*OpenerDTO, error) {
	o := h.service.FindByName(input.Name)
	return toOpenerDTO(o), nil
}

// OpenerOpenInput open 接口入参。huma 约定：请求体字段须挂在名为 Body 的子结构上。
type OpenerOpenInput struct {
	Body struct {
		Path   string `json:"path" doc:"文件或目录绝对路径"`
		Opener string `json:"opener" doc:"opener 名称"`
	}
}

// openerOpen 用指定 opener 打开任意文件/目录（项目打开走 project/open，那里记 usage；此处不产生使用信号）。
// role 按路径实际类型校验：目录须 open-dir、文件须 open-file。
func (h *OpenerHandler) openerOpen(input OpenerOpenInput) (map[string]any, error) {
	if !filepath.IsAbs(input.Body.Path) {
		return nil, errors.New("path 必须是绝对路径: " + input.Body.Path)
	}
	info, err := os.Stat(input.Body.Path)
	if err != nil {
		return nil, fmt.Errorf("读取路径失败: %w", err)
	}

	o := h.service.FindByName(input.Body.Opener)
	if o == nil {
		return nil, fmt.Errorf("未找到指定 opener: %s", input.Body.Opener)
	}
	role := opener.RoleOpenFile
	if info.IsDir() {
		role = opener.RoleOpenDir
	}

	// role 是否支持由实现内校验（Open 首步），这里只透传错误
	if err := o.Open(role, input.Body.Path); err != nil {
		return nil, fmt.Errorf("打开失败: %w", err)
	}
	return map[string]any{"ok": true}, nil
}

// OpenerDiffOpenInput diff-open 接口入参。
type OpenerDiffOpenInput struct {
	Body struct {
		Left   string `json:"left" doc:"左侧路径（文件或目录）绝对路径"`
		Right  string `json:"right" doc:"右侧路径（文件或目录）绝对路径"`
		Opener string `json:"opener" doc:"opener 名称"`
	}
}

// openerDiffOpen 用指定 opener 对比两个路径。role 按两侧实际类型推导且须同类
// （两侧都是目录 → diff-dir，都是文件 → diff-file，混合报错）。
func (h *OpenerHandler) openerDiffOpen(input OpenerDiffOpenInput) (map[string]any, error) {
	left, right := input.Body.Left, input.Body.Right
	for _, p := range []string{left, right} {
		if !filepath.IsAbs(p) {
			return nil, errors.New("路径必须是绝对路径: " + p)
		}
	}
	leftDir, err := isDirPath(left)
	if err != nil {
		return nil, fmt.Errorf("读取路径失败: %w", err)
	}
	rightDir, err := isDirPath(right)
	if err != nil {
		return nil, fmt.Errorf("读取路径失败: %w", err)
	}
	if leftDir != rightDir {
		return nil, errors.New("两侧路径类型不一致（一边是目录一边是文件），无法对比")
	}

	o := h.service.FindByName(input.Body.Opener)
	if o == nil {
		return nil, fmt.Errorf("未找到指定 opener: %s", input.Body.Opener)
	}
	role := opener.RoleDiffFile
	if leftDir {
		role = opener.RoleDiffDir
	}
	if err := o.Open(role, left, right); err != nil {
		return nil, fmt.Errorf("打开失败: %w", err)
	}
	return map[string]any{"ok": true}, nil
}

func isDirPath(p string) (bool, error) {
	info, err := os.Stat(p)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// OpenerSaveInput save 接口入参（字段与 opener.Spec 对齐）。
type OpenerSaveInput struct {
	Body struct {
		Name    string            `json:"name" doc:"opener 名称（唯一标识）"`
		Title   string            `json:"title,omitempty" doc:"展示文案（如「打开所在目录」），缺省由 name 生成"`
		Actions map[string]string `json:"actions" doc:"role → 动作串，exec: 命令 / url: 链接，$0/$1 占位路径槽位"`
		Icon    *IconDTO          `json:"icon,omitempty" doc:"图标声明"`
	}
}

func (h *OpenerHandler) openerSave(input OpenerSaveInput) (map[string]any, error) {
	actions := make(map[opener.Role]string, len(input.Body.Actions))
	for r, raw := range input.Body.Actions {
		actions[opener.Role(r)] = raw
	}
	spec := opener.Spec{
		Name:    input.Body.Name,
		Title:   input.Body.Title,
		Actions: actions,
	}
	if input.Body.Icon != nil {
		spec.Icon = &opener.Icon{Type: input.Body.Icon.Type, Value: input.Body.Icon.Value}
	}
	// 校验在 Service 写侧（领域构造函数），坏数据返回中文错误、不落文件
	if err := h.service.SaveOpener(spec); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// OpenerDeleteInput delete 接口入参。
type OpenerDeleteInput struct {
	Body struct {
		Name string `json:"name" doc:"opener 名称"`
	}
}

func (h *OpenerHandler) openerDelete(input OpenerDeleteInput) (map[string]any, error) {
	if err := h.service.DeleteOpener(input.Body.Name); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// OpenerReorderInput reorder 接口入参（整表按目标顺序提交名单）。
type OpenerReorderInput struct {
	Body struct {
		Names []string `json:"names" doc:"按目标顺序排列的 opener 名单"`
	}
}

func (h *OpenerHandler) openerReorder(input OpenerReorderInput) (map[string]any, error) {
	if err := h.service.ReorderOpeners(input.Body.Names); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// IntentDefaultSaveInput intent-default/save 接口入参。
type IntentDefaultSaveInput struct {
	Body struct {
		Intent string `json:"intent" doc:"打开意图（如 dir / file / diff-file / git）"`
		Opener string `json:"opener" doc:"opener 名称（须声明该 intent 对应的 role）"`
	}
}

func (h *OpenerHandler) intentDefaultSave(input IntentDefaultSaveInput) (map[string]any, error) {
	intent, err := opener.ParseIntent(input.Body.Intent)
	if err != nil {
		return nil, err
	}
	if err := h.service.SaveIntentDefault(intent, input.Body.Opener); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// IntentDefaultDeleteInput intent-default/delete 接口入参。
type IntentDefaultDeleteInput struct {
	Body struct {
		Intent string `json:"intent" doc:"打开意图"`
	}
}

func (h *OpenerHandler) intentDefaultDelete(input IntentDefaultDeleteInput) (map[string]any, error) {
	intent, err := opener.ParseIntent(input.Body.Intent)
	if err != nil {
		return nil, err
	}
	if err := h.service.DeleteIntentDefault(intent); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}
