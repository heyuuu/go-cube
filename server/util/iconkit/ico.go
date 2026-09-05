// ICO 容器解析：favicon.ico 的主流形态，标准库不支持，此处手解。
// 只支持现代站点实际产出的子集：PNG 内嵌条目 + BI_RGB 未压缩 DIB（24/32bpp）。
package iconkit

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// isIco ICO 魔数：reserved=0（2B）+ type=1 图标（2B）。
func isIco(data []byte) bool {
	return len(data) >= 6 &&
		data[0] == 0 && data[1] == 0 &&
		data[2] == 1 && data[3] == 0
}

// decodeIco 解码 ICO 容器。条目选择：优先取「最长边 ≤64 的最大条目」（原生小图
// 免缩放、比 256px 大图缩小质量更好），无则退回全局最大条目。
func decodeIco(data []byte) (image.Image, error) {
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count == 0 {
		return nil, fmt.Errorf("ICO 条目数为 0")
	}

	var best []byte
	var bestDim int
	var smallBest []byte
	var smallBestDim int
	for i := 0; i < count; i++ {
		off := 6 + i*16
		if off+16 > len(data) {
			return nil, fmt.Errorf("ICO 目录越界: entry=%d", i)
		}
		e := data[off:]
		// 目录字段：宽(1B，0=256) 高(1B，0=256) … bytesInRes(4B) imageOffset(4B)
		dim := int(e[0])
		if dim == 0 {
			dim = 256
		}
		size := int(binary.LittleEndian.Uint32(e[8:12]))
		imgOff := int(binary.LittleEndian.Uint32(e[12:16]))
		if imgOff < 0 || size < 0 || imgOff+size > len(data) {
			return nil, fmt.Errorf("ICO 条目数据越界: entry=%d offset=%d size=%d", i, imgOff, size)
		}
		payload := data[imgOff : imgOff+size]
		if dim <= 64 {
			if dim > smallBestDim {
				smallBest, smallBestDim = payload, dim
			}
		} else if dim > bestDim {
			best, bestDim = payload, dim
		}
	}

	if smallBest != nil {
		best = smallBest
	}
	if best == nil {
		return nil, fmt.Errorf("ICO 内无可用条目")
	}

	if bytes.HasPrefix(best, pngMagic) {
		img, err := png.Decode(bytes.NewReader(best))
		if err != nil {
			return nil, fmt.Errorf("ICO 内 PNG 条目解码失败: %w", err)
		}
		return img, nil
	}
	return decodeIcoDib(best)
}

// decodeIcoDib 解码条目内的 DIB 位图（BITMAPINFOHEADER + XOR 位图 + AND 掩码）。
// 支持子集：planes=1、BI_RGB 未压缩、24/32bpp；biHeight 是「XOR + AND」双区高度需减半。
func decodeIcoDib(dib []byte) (image.Image, error) {
	if len(dib) < 40 {
		return nil, fmt.Errorf("ICO DIB 头不完整")
	}
	biWidth := int(int32(binary.LittleEndian.Uint32(dib[4:8])))
	biHeight := int(int32(binary.LittleEndian.Uint32(dib[8:12])))
	planes := int(binary.LittleEndian.Uint16(dib[12:14]))
	bitCount := int(binary.LittleEndian.Uint16(dib[14:16]))
	compression := uint32(binary.LittleEndian.Uint32(dib[16:20]))

	if planes != 1 {
		return nil, fmt.Errorf("ICO DIB planes=%d 不支持（仅 1）", planes)
	}
	if compression != 0 {
		return nil, fmt.Errorf("ICO DIB compression=%d 不支持（仅 BI_RGB 未压缩）", compression)
	}
	if bitCount != 24 && bitCount != 32 {
		return nil, fmt.Errorf("ICO DIB bitCount=%d 不支持（仅 24/32bpp）", bitCount)
	}
	if biWidth <= 0 || biHeight <= 0 || biHeight%2 != 0 {
		return nil, fmt.Errorf("ICO DIB 尺寸异常: w=%d h=%d", biWidth, biHeight)
	}
	h := biHeight / 2
	w := biWidth

	stride := (w*bitCount/8 + 3) / 4 * 4 // XOR 行距（4 字节对齐）
	xorLen := stride * h
	maskStride := (w + 31) / 32 * 4 // AND 掩码行距（1bpp）
	maskLen := maskStride * h
	if 40+xorLen > len(dib) {
		return nil, fmt.Errorf("ICO DIB 数据不完整: need=%d got=%d", 40+xorLen, len(dib))
	}
	hasMask := 40+xorLen+maskLen <= len(dib)

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	allAlphaZero := true
	for row := 0; row < h; row++ {
		src := dib[40+row*stride:]
		y := h - 1 - row // DIB 行序自底向上
		for x := 0; x < w; x++ {
			var c color.RGBA
			if bitCount == 32 {
				p := src[x*4:]
				c = color.RGBA{R: p[2], G: p[1], B: p[0], A: p[3]} // BGRA → RGBA
				if p[3] != 0 {
					allAlphaZero = false
				}
			} else {
				p := src[x*3:]
				c = color.RGBA{R: p[2], G: p[1], B: p[0], A: 0xFF}
			}
			img.SetRGBA(x, y, c)
		}
	}

	// 32bpp 全零 alpha 是常见的历史兼容产物（透明信息实际在 AND 掩码里），
	// 直接采用会把整图标渲染成透明——按掩码回填不透明：bit=0 表示非透明。
	if bitCount == 32 && allAlphaZero && hasMask {
		for row := 0; row < h; row++ {
			src := dib[40+xorLen+row*maskStride:]
			y := h - 1 - row
			for x := 0; x < w; x++ {
				if src[x/8]&(0x80>>(x%8)) == 0 {
					c := img.RGBAAt(x, y)
					c.A = 0xFF
					img.SetRGBA(x, y, c)
				}
			}
		}
	}
	return img, nil
}
