package iconkit

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cube/internal/testfixture"
)

// makeColorPng 生成 size×size 的指定色 PNG，供容器类测试当 payload（makePng 是固定色的既有版本）。
func makeColorPng(t *testing.T, size int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatalf("构造测试 PNG 失败: %v", err)
	}
	return out.Bytes()
}

// buildIco 构造 ICO 容器：entries 为 (边长, payload) 序列，payload 须自备格式（PNG/DIB）。
func buildIco(t *testing.T, entries []struct {
	dim     int
	payload []byte
}) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.Write([]byte{0, 0, 1, 0})
	binary.Write(&buf, binary.LittleEndian, uint16(len(entries)))
	offset := 6 + 16*len(entries)
	for _, e := range entries {
		dim := e.dim
		if dim == 256 {
			dim = 0
		}
		buf.WriteByte(byte(dim))
		buf.WriteByte(byte(dim))
		buf.Write([]byte{0, 0, 1, 0}) // colorCount, reserved, planes=1
		binary.Write(&buf, binary.LittleEndian, uint16(32))
		binary.Write(&buf, binary.LittleEndian, uint32(len(e.payload)))
		binary.Write(&buf, binary.LittleEndian, uint32(offset))
		offset += len(e.payload)
	}
	for _, e := range entries {
		buf.Write(e.payload)
	}
	return buf.Bytes()
}

// buildDib 构造 32bpp BI_RGB DIB（2x2），行序自底向上；alpha 全零 + AND 掩码由调用方追加。
func buildDib(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(40)) // biSize
	binary.Write(&buf, binary.LittleEndian, int32(2))   // biWidth
	binary.Write(&buf, binary.LittleEndian, int32(4))   // biHeight = XOR(2) + AND(2)
	binary.Write(&buf, binary.LittleEndian, uint16(1))  // planes
	binary.Write(&buf, binary.LittleEndian, uint16(32)) // bitCount
	binary.Write(&buf, binary.LittleEndian, uint32(0))  // BI_RGB
	binary.Write(&buf, binary.LittleEndian, uint32(0))  // biSizeImage
	binary.Write(&buf, binary.LittleEndian, uint32(0))  // xpels
	binary.Write(&buf, binary.LittleEndian, uint32(0))  // ypels
	binary.Write(&buf, binary.LittleEndian, uint32(0))  // biClrUsed
	binary.Write(&buf, binary.LittleEndian, uint32(0))  // biClrImportant
	// XOR：行 0（底部）：BGRA 红(0,0,255) ×2；行 1（顶部）：BGRA 绿(0,255,0) ×2（字节序 B,G,R,A）
	for _, px := range [4][4]byte{{0, 0, 255, 255}, {0, 0, 255, 255}, {0, 255, 0, 255}, {0, 255, 0, 255}} {
		buf.Write(px[:])
	}
	return buf.Bytes()
}

// TestDecodeImage_PngInIco ICO 内嵌 PNG 条目解码 + 条目选择（≤64 优先）。
func TestDecodeImage_PngInIco(t *testing.T) {
	red := makeColorPng(t, 32, color.RGBA{R: 255, A: 255})
	blue := makeColorPng(t, 48, color.RGBA{B: 255, A: 255})
	ico := buildIco(t, []struct {
		dim     int
		payload []byte
	}{{32, red}, {48, blue}})

	img, err := DecodeImage(ico)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 48 || b.Dy() != 48 {
		t.Fatalf("应选 48px 条目, got %dx%d", b.Dx(), b.Dy())
	}
	r, g, bl, _ := img.At(24, 24).RGBA()
	if r != 0 || g != 0 || bl == 0 {
		t.Fatalf("应取 48px 的蓝色条目, got r=%d g=%d b=%d", r, g, bl)
	}
}

// TestDecodeImage_Dib32 ICO DIB 32bpp：BGRA→RGBA 换序 + 自底向上行序 + 全零 alpha 掩码回填。
func TestDecodeImage_Dib32(t *testing.T) {
	dib := buildDib(t)
	// AND 掩码 2 行 × 4B：全部透明位（0xFF...）→ 回填后全部不透明；用 0x00 验证非透明路径
	mask := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	// alpha 全零：重建 XOR 区把 alpha 字节清零
	for i := 40; i < len(dib); i += 4 {
		dib[i+3] = 0
	}
	payload := append(append([]byte{}, dib...), mask...)
	ico := buildIco(t, []struct {
		dim     int
		payload []byte
	}{{2, payload}})

	img, err := DecodeImage(ico)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 2 {
		t.Fatalf("尺寸应 2x2, got %v", img.Bounds())
	}
	// 底部 (0,1)（= DIB 存储行 0，bottom-up 首行）应为红色，且 alpha 经掩码回填为不透明
	r, g, b, a := img.At(0, 1).RGBA()
	if r == 0 || g != 0 || b != 0 || a != 0xFFFF {
		t.Fatalf("底部应为不透明红色, got r=%d g=%d b=%d a=%d", r, g, b, a)
	}
	// 顶部 (0,0)（= DIB 存储行 1）应为绿色
	r, g, b, _ = img.At(0, 0).RGBA()
	if r != 0 || g == 0 || b != 0 {
		t.Fatalf("顶部应为绿色, got r=%d g=%d b=%d", r, g, b)
	}
}

// TestDecodeImage_Unsupported 非图片字节返回中文错误。
func TestDecodeImage_Unsupported(t *testing.T) {
	if _, err := DecodeImage([]byte("not an image")); err == nil || !strings.Contains(err.Error(), "无法识别") {
		t.Fatalf("应报中文错误, got %v", err)
	}
}

// TestExtractFromSource 本地图片文件与目录（.app）分发、非存在路径报错。
func TestExtractFromSource(t *testing.T) {
	ws := testfixture.NewWorkspace(t)

	pngPath := filepath.Join(ws.Dir, "icon.png")
	if err := os.WriteFile(pngPath, makeColorPng(t, 100, color.RGBA{G: 255, A: 255}), 0o644); err != nil {
		t.Fatalf("写测试图片失败: %v", err)
	}
	data, err := ExtractFromSource(pngPath)
	if err != nil {
		t.Fatalf("本地图片提取失败: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("提取结果应为 PNG: %v", err)
	}
	if img.Bounds().Dx() != 64 {
		t.Fatalf("100px 应缩放到 64, got %d", img.Bounds().Dx())
	}

	// .app 目录分发：走 ExtractAppIcon（容器合法性由其自身测试覆盖，此处验证分发）
	resDir := ws.Join("Fake.app", "Contents", "Resources")
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	icns := makeIcns(t, map[string][]byte{"ic07": makeColorPng(t, 16, color.RGBA{R: 255, A: 255})})
	if err := os.WriteFile(filepath.Join(resDir, "Fake.icns"), icns, 0o644); err != nil {
		t.Fatalf("写 icns 失败: %v", err)
	}
	if _, err := ExtractFromSource(ws.Join("Fake.app")); err != nil {
		t.Fatalf(".app 目录应分发到 icns 提取: %v", err)
	}

	if _, err := ExtractFromSource("/no/such/path.png"); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("不存在路径应报中文错误, got %v", err)
	}
}

// TestExtractFromSource_URL URL 分发：httptest 服务 PNG 与 ICO（favicon 场景）。
func TestExtractFromSource_URL(t *testing.T) {
	red := makeColorPng(t, 32, color.RGBA{R: 255, A: 255})
	ico := buildIco(t, []struct {
		dim     int
		payload []byte
	}{{32, red}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/favicon.ico":
			w.Write(ico)
		case "/missing":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.Write(red)
		}
	}))
	t.Cleanup(srv.Close)

	for _, u := range []string{srv.URL + "/favicon.ico", srv.URL + "/icon.png"} {
		data, err := ExtractFromSource(u)
		if err != nil {
			t.Fatalf("URL 提取失败: %s err=%v", u, err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("提取结果应为 PNG: %s err=%v", u, err)
		}
		if img.Bounds().Dx() != 32 {
			t.Fatalf("尺寸应保持 32, got %d", img.Bounds().Dx())
		}
	}

	// 非 200 / 非图片 / 非 http 协议
	if _, err := ExtractFromSource(srv.URL + "/missing"); err == nil {
		t.Fatalf("404 应报错")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not an image"))
	}))
	t.Cleanup(bad.Close)
	if _, err := ExtractFromSource(bad.URL); err == nil || !strings.Contains(err.Error(), "不是支持的图片") {
		t.Fatalf("非图片 URL 应报中文错误, got %v", err)
	}
	if _, err := ExtractFromSource("ftp://example.com/x.png"); err == nil || !strings.Contains(err.Error(), "不支持的图标来源协议") {
		t.Fatalf("非 http 协议应报中文错误, got %v", err)
	}
}
