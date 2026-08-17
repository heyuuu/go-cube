package web

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// md 渲染提案（docs/proposals/260811-md渲染）定稿：后端不做任何模板渲染，
// 只提供「读本地 markdown 原文」这唯一一个 API；渲染、样式、交互全归前端工程。

// MdContentResult md 原文接口返回结构。
type MdContentResult struct {
	Content string `json:"content"` // markdown 原文（纯文本，未渲染）
}

// MdListResult 目录浏览接口返回结构：根下全部 .md 文件绝对路径（字典序）。
type MdListResult struct {
	Dir   bool     `json:"dir"`   // path 是否目录（false = 单文件模式，Files 仅自身）
	Files []string `json:"files"` // .md 文件绝对路径
}

type MdHandler struct{}

func NewMdHandler() *MdHandler {
	return &MdHandler{}
}

func (h *MdHandler) Register(api huma.API) {
	apiGet(api, "/api/md/content", "读取 markdown 文件原文", h.mdContent)
	apiGet(api, "/api/md/list", "列出目录下的 markdown 文件", h.mdList)
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

func (h *MdHandler) mdList(input struct {
	Path string `query:"path" required:"true"`
}) (MdListResult, error) {
	if !filepath.IsAbs(input.Path) {
		return MdListResult{}, errors.New("path 必须是绝对路径: " + input.Path)
	}
	info, err := os.Stat(input.Path)
	if err != nil {
		return MdListResult{}, fmt.Errorf("读取路径失败: %w", err)
	}
	if !info.IsDir() {
		// 单文件模式：Files 仅自身，前端据此走无侧栏形态
		return MdListResult{Dir: false, Files: []string{input.Path}}, nil
	}
	return MdListResult{Dir: true, Files: walkMdFiles(input.Path)}, nil
}

// walkMdFiles 递归收集 root 下全部 .md 文件（扩展名不区分大小写），WalkDir 天然字典序。
// 跳过隐藏目录与 node_modules（对文档浏览是噪音）；单目录读取失败降级跳过，不中断整体。
func walkMdFiles(root string) []string {
	var files []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 降级：读不了的目录跳过
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			files = append(files, p)
		}
		return nil
	})
	return files
}
