package iconkit

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// makePng 生成 w×h 的纯色 PNG。
func makePng(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 0x40, G: 0x80, B: 0xc0, A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("生成 PNG 失败: %v", err)
	}
	return buf.Bytes()
}

// makeIcns 组装一个含若干 PNG 条目的 icns 容器。
func makeIcns(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var body []byte
	for typ, payload := range entries {
		entry := make([]byte, 8+len(payload))
		copy(entry, typ)
		binary.BigEndian.PutUint32(entry[4:8], uint32(len(entry)))
		copy(entry[8:], payload)
		body = append(body, entry...)
	}
	header := make([]byte, 8)
	copy(header, "icns")
	binary.BigEndian.PutUint32(header[4:8], uint32(8+len(body)))
	return append(header, body...)
}

func TestLargestPngEntry(t *testing.T) {
	t.Run("取最大 PNG 条目", func(t *testing.T) {
		small, big := makePng(t, 16, 16), makePng(t, 128, 128)
		icns := makeIcns(t, map[string][]byte{"ic07": small, "ic09": big})
		got, err := largestPngEntry(icns)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if !bytes.Equal(got, big) {
			t.Fatal("应取最大条目")
		}
	})

	t.Run("非 PNG 条目被跳过", func(t *testing.T) {
		png1 := makePng(t, 32, 32)
		icns := makeIcns(t, map[string][]byte{"is32": []byte("raw argby bytes")})
		if _, err := largestPngEntry(icns); err == nil {
			t.Fatal("无 PNG 条目应报错")
		}
		_ = png1
	})

	t.Run("非 icns 文件报错", func(t *testing.T) {
		if _, err := largestPngEntry([]byte("not icns at all")); err == nil {
			t.Fatal("应报错")
		}
	})
}

func TestScalePng(t *testing.T) {
	t.Run("长边缩到 max", func(t *testing.T) {
		got, err := ScalePng(makePng(t, 256, 128), 64)
		if err != nil {
			t.Fatalf("缩放失败: %v", err)
		}
		img, err := png.Decode(bytes.NewReader(got))
		if err != nil {
			t.Fatalf("结果不是合法 PNG: %v", err)
		}
		if w, h := img.Bounds().Dx(), img.Bounds().Dy(); w != 64 || h != 32 {
			t.Fatalf("期望 64x32（等比）, got %dx%d", w, h)
		}
	})

	t.Run("不超 max 原样返回", func(t *testing.T) {
		src := makePng(t, 32, 32)
		got, err := ScalePng(src, 64)
		if err != nil {
			t.Fatalf("失败: %v", err)
		}
		if !bytes.Equal(src, got) {
			t.Fatal("小图应原样返回")
		}
	})
}

func TestExtractAppIcon(t *testing.T) {
	dir := t.TempDir()
	resDir := filepath.Join(dir, "Foo.app", "Contents", "Resources")
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(dir, "Foo.app")

	writeIcns := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(resDir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("正常提取并缩放", func(t *testing.T) {
		writeIcns("AppIcon.icns", makeIcns(t, map[string][]byte{"ic08": makePng(t, 256, 256)}))
		got, err := ExtractAppIcon(appDir)
		if err != nil {
			t.Fatalf("提取失败: %v", err)
		}
		img, err := png.Decode(bytes.NewReader(got))
		if err != nil {
			t.Fatalf("结果不是合法 PNG: %v", err)
		}
		if w := img.Bounds().Dx(); w != 64 {
			t.Fatalf("应缩到 64, got %d", w)
		}
	})

	t.Run("多个 icns 取最大文件", func(t *testing.T) {
		writeIcns("Small.icns", makeIcns(t, map[string][]byte{"ic07": makePng(t, 128, 128)}))
		writeIcns("Big.icns", makeIcns(t, map[string][]byte{"ic10": makePng(t, 512, 512)}))
		got, err := ExtractAppIcon(appDir)
		if err != nil {
			t.Fatalf("提取失败: %v", err)
		}
		if img, _ := png.Decode(bytes.NewReader(got)); img.Bounds().Dx() != 64 {
			t.Fatal("应从最大的 icns 缩放到 64")
		}
	})

	t.Run("无 icns 报错", func(t *testing.T) {
		barRes := filepath.Join(dir, "Bar.app", "Contents", "Resources")
		if err := os.MkdirAll(barRes, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := ExtractAppIcon(filepath.Join(dir, "Bar.app")); err == nil {
			t.Fatal("无 icns 应报错")
		}
	})
}
