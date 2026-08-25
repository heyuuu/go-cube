// Package iconkit 从 macOS .app 包提取图标（.app → icns → PNG），
// 供 opener 配置页「从本地 .app 选图」入口使用（1016）。
//
// 提取结果是缩放到 ≤64px 的 PNG 字节，调用方 base64 后存入 icon 声明。
// 纯函数、无环境副作用：只依赖入参路径与字节运算。
// 非 macOS 平台的 .app 包结构相同，本包同样可用（「darwin-only」指使用场景，
// 无需 build tag）。
package iconkit

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// maxSize 提取结果的最长边（px）。图标展示场景 64px 足够，更大的只浪费存储。
const maxSize = 64

// pngMagic PNG 文件头。
var pngMagic = []byte{0x89, 'P', 'N', 'G'}

// ExtractAppIcon 从 .app 目录提取图标，返回缩放到 ≤64px 的 PNG 字节。
//
// icns 定位策略：不解析（通常是二进制格式的）Info.plist，直接扫 Contents/Resources
// 下全部 *.icns 取最大者——多数 .app 只有一个 icns，多图时取最大的清晰度最好。
func ExtractAppIcon(appPath string) ([]byte, error) {
	resDir := filepath.Join(appPath, "Contents", "Resources")
	entries, err := os.ReadDir(resDir)
	if err != nil {
		return nil, fmt.Errorf("读取 .app 资源目录失败: dir=%s err=%w", resDir, err)
	}

	var bestPath string
	var bestSize int64
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".icns") {
			continue
		}
		if info, err := e.Info(); err == nil && info.Size() > bestSize {
			bestPath, bestSize = filepath.Join(resDir, e.Name()), info.Size()
		}
	}
	if bestPath == "" {
		return nil, fmt.Errorf(".app 内未找到 icns 图标: app=%s", appPath)
	}

	data, err := os.ReadFile(bestPath)
	if err != nil {
		return nil, fmt.Errorf("读取 icns 失败: file=%s err=%w", bestPath, err)
	}
	pngData, err := largestPngEntry(data)
	if err != nil {
		return nil, fmt.Errorf("解析 icns 失败: file=%s err=%w", bestPath, err)
	}
	return ScalePng(pngData, maxSize)
}

// largestPngEntry 从 icns 容器中挑出最大尺寸的 PNG 条目。
//
// icns 格式：文件头 4 字节 "icns" + 4 字节总长（大端），随后为条目序列——
// 每条 4 字节类型码（如 ic07/ic08/ic10）+ 4 字节条目总长（含头 8 字节）。
// 现代条目的 payload 本身就是 PNG/JPEG2000；此处只取 PNG magic 命中的条目
// （老式 ARGB 原始位图条目不含尺寸与像素格式细节，不值得手解，遇到即跳过）。
func largestPngEntry(data []byte) ([]byte, error) {
	if len(data) < 8 || string(data[:4]) != "icns" {
		return nil, fmt.Errorf("不是合法的 icns 文件")
	}
	var best []byte
	for off := 8; off+8 <= len(data); {
		entryLen := int(binary.BigEndian.Uint32(data[off+4 : off+8]))
		if entryLen < 8 || off+entryLen > len(data) {
			return nil, fmt.Errorf("icns 条目长度异常: offset=%d len=%d", off, entryLen)
		}
		payload := data[off+8 : off+entryLen]
		if bytes.HasPrefix(payload, pngMagic) && len(payload) > len(best) {
			best = payload
		}
		off += entryLen
	}
	if best == nil {
		return nil, fmt.Errorf("icns 内无 PNG 条目（可能仅含老式 ARGB 位图）")
	}
	return best, nil
}

// ScalePng 把 PNG 解码后等比缩放到最长边 ≤max（nearest 邻采样——图标缩放
// 质量足够，避免引 x/image 依赖），再编码回 PNG 字节。原图不长于 max 时原样返回。
func ScalePng(data []byte, max int) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("解码 PNG 失败: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= max && h <= max {
		return data, nil
	}

	// 等比缩到最长边 = max
	nw, nh := w, h
	if w >= h {
		nw, nh = max, max*h/w
	} else {
		nw, nh = max*w/h, max
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := b.Min.Y + y*h/nh
		for x := 0; x < nw; x++ {
			sx := b.Min.X + x*w/nw
			dst.Set(x, y, img.At(sx, sy))
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, fmt.Errorf("编码 PNG 失败: %w", err)
	}
	return out.Bytes(), nil
}
