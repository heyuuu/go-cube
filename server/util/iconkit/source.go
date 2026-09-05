// 从任意源（本地路径 / URL）提取图标：IconField「提取」入口的领域分发 + 图片字节解码。
// 与 iconkit.go 的 .app/icns 提取同处一个包，输出口径一致（≤64px PNG）。
package iconkit

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// maxFetchSize URL 拉取的字节上限：favicon/小图场景 4MB 远超所需，防误填大文件链接拖内存。
const maxFetchSize = 4 << 20

// ExtractFromSource 从路径或 URL 提取图标，返回缩放到 ≤64px 的 PNG 字节：
//   - http(s):// URL → 拉取字节后按图片解码（favicon.ico / png / jpg / gif）；
//   - 本地目录（.app）→ ExtractAppIcon（icns 容器）；
//   - 本地文件 → 读字节后按图片解码（png / ico / jpg / gif）。
func ExtractFromSource(source string) ([]byte, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("图标来源不得为空")
	}

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		data, err := FetchImage(source)
		if err != nil {
			return nil, err
		}
		img, err := DecodeImage(data)
		if err != nil {
			return nil, fmt.Errorf("URL 内容不是支持的图片: %s", source)
		}
		return EncodeImageScaled(img, maxSize)
	}

	if strings.Contains(source, "://") {
		return nil, fmt.Errorf("不支持的图标来源协议（仅 http/https URL 或本地路径）: %s", source)
	}

	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("图标来源路径不存在: %s", source)
	}
	if info.IsDir() {
		return ExtractAppIcon(source)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("读取图标文件失败: file=%s err=%w", source, err)
	}
	img, err := DecodeImage(data)
	if err != nil {
		return nil, fmt.Errorf("文件不是支持的图片: %s", source)
	}
	return EncodeImageScaled(img, maxSize)
}

// FetchImage 拉取 URL 内容字节（favicon 等小图场景；非 200 状态返回中文错误）。
func FetchImage(url string) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求图标 URL 失败: url=%s err=%w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("图标 URL 返回非 200: url=%s status=%d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchSize+1))
	if err != nil {
		return nil, fmt.Errorf("读取图标 URL 响应失败: url=%s err=%w", url, err)
	}
	if len(data) > maxFetchSize {
		return nil, fmt.Errorf("图标 URL 内容超过 4MB 上限: url=%s", url)
	}
	return data, nil
}

// DecodeImage 按魔数分发解码 PNG / JPEG / GIF / ICO 字节。
// ICO 是 favicon 的主流形态而标准库不支持，容器解析见 ico.go。
func DecodeImage(data []byte) (image.Image, error) {
	switch {
	case bytes.HasPrefix(data, pngMagic):
		return png.Decode(bytes.NewReader(data))
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return jpeg.Decode(bytes.NewReader(data))
	case len(data) >= 4 && string(data[:4]) == "GIF8":
		return gif.Decode(bytes.NewReader(data))
	case isIco(data):
		return decodeIco(data)
	}
	return nil, fmt.Errorf("无法识别的图片格式（支持 PNG / JPEG / GIF / ICO）")
}

// EncodeImageScaled 把已解码图像等比缩放到最长边 ≤max 并编码为 PNG 字节
// （不长于 max 时直接编码，不放大；与 ScalePng 共享同一缩放实现）。
func EncodeImageScaled(img image.Image, max int) ([]byte, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= max && h <= max {
		var out bytes.Buffer
		if err := png.Encode(&out, img); err != nil {
			return nil, fmt.Errorf("编码 PNG 失败: %w", err)
		}
		return out.Bytes(), nil
	}

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
