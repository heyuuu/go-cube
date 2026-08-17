package web

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"
)

// md 渲染提案（docs/proposals/260811-md渲染）定稿：后端不做任何模板渲染，
// 只提供「读本地 markdown 原文」这唯一一个 API；渲染、样式、交互全归前端工程。

// MdContentResult md 原文接口返回结构。
type MdContentResult struct {
	Content string `json:"content"` // markdown 原文（纯文本，未渲染）
}

type MdHandler struct{}

func NewMdHandler() *MdHandler {
	return &MdHandler{}
}

func (h *MdHandler) Register(api huma.API) {
	apiGet(api, "/api/md/content", "读取 markdown 文件原文", h.mdContent)
}

func (h *MdHandler) mdContent(input struct {
	Path string `query:"path" required:"true"`
}) (MdContentResult, error) {
	if !filepath.IsAbs(input.Path) {
		return MdContentResult{}, errors.New("path 必须是绝对路径: " + input.Path)
	}
	data, err := os.ReadFile(input.Path)
	if err != nil {
		return MdContentResult{}, fmt.Errorf("读取文件失败: %w", err)
	}
	return MdContentResult{Content: string(data)}, nil
}
