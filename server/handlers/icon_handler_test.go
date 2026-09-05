package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

// postRaw 打 POST 并断言 200 + envelope，返回 envelope（icon_handler 用例自带，
// 不依赖 workbench_test 的 postJSON 也行——保持同构直接复用）。
func postRaw(t *testing.T, url string, body string) envelope {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST %s 失败: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s 应为 200, got %d", url, resp.StatusCode)
	}
	var out envelope
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("响应解码失败: %v", err)
	}
	return out
}

// makeTestPng 构造纯色 PNG 字节。
func makeTestPng(t *testing.T, size int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatalf("构造 PNG 失败: %v", err)
	}
	return out.Bytes()
}

// buildTestIcns 构造含单 PNG 条目的 icns 容器（最小可用形态）。
func buildTestIcns(pngData []byte) []byte {
	entry := make([]byte, 8+len(pngData))
	copy(entry, "ic07")
	binary.BigEndian.PutUint32(entry[4:8], uint32(len(entry)))
	copy(entry[8:], pngData)
	icns := make([]byte, 8, 8+len(entry))
	copy(icns, "icns")
	binary.BigEndian.PutUint32(icns[4:8], uint32(8+len(entry)))
	return append(icns, entry...)
}

// buildTestIco 构造含单 PNG 条目的 ICO 容器。
func buildTestIco(pngData []byte, dim int) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0, 0, 1, 0, 1, 0})
	buf.WriteByte(byte(dim))
	buf.WriteByte(byte(dim))
	buf.Write([]byte{0, 0, 1, 0}) // colorCount, reserved, planes=1
	binary.Write(&buf, binary.LittleEndian, uint16(32))
	binary.Write(&buf, binary.LittleEndian, uint32(len(pngData)))
	binary.Write(&buf, binary.LittleEndian, uint32(22)) // 6 头 + 16 目录
	buf.Write(pngData)
	return buf.Bytes()
}

// TestIconExtract 三种 source 形态的出口契约：本地图片文件 / .app 目录 / http URL。
func TestIconExtract(t *testing.T) {
	env := newTestEnv(t)
	pngData := makeTestPng(t, 32, color.RGBA{R: 255, A: 255})
	env.ws.WriteFile("icon.png", pngData)

	// 本地图片文件
	out := postRaw(t, env.url("/api/icon/extract"), `{"source":`+quote(env.ws.Join("icon.png"))+`}`)
	if !out.Ok {
		t.Fatalf("本地文件提取应成功, message=%q", out.Message)
	}
	decodeIconValue(t, out)

	// .app 目录（icns 容器合法性由 iconkit 测试覆盖，走真实分发）
	env.ws.Mkdir("Fake.app", "Contents", "Resources")
	env.ws.WriteFile("Fake.app/Contents/Resources/Fake.icns", buildTestIcns(pngData))
	out = postRaw(t, env.url("/api/icon/extract"), `{"source":`+quote(env.ws.Join("Fake.app"))+`}`)
	if !out.Ok {
		t.Fatalf(".app 提取应成功, message=%q", out.Message)
	}
	decodeIconValue(t, out)

	// URL（favicon.ico 场景：ICO 内嵌 PNG）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(buildTestIco(pngData, 32))
	}))
	t.Cleanup(srv.Close)
	out = postRaw(t, env.url("/api/icon/extract"), `{"source":`+quote(srv.URL+"/favicon.ico")+`}`)
	if !out.Ok {
		t.Fatalf("URL 提取应成功, message=%q", out.Message)
	}
	decodeIconValue(t, out)

	// 坏 source：中文错误
	bad := postRaw(t, env.url("/api/icon/extract"), `{"source":"/no/such/file.png"}`)
	if bad.Ok || !bytes.Contains([]byte(bad.Message), []byte("不存在")) {
		t.Fatalf("坏路径应报中文错误, message=%q", bad.Message)
	}
}

// quote JSON 字符串字面量包装。
func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// decodeIconValue 断言 envelope.data.value 是合法 base64 PNG。
func decodeIconValue(t *testing.T, env envelope) {
	t.Helper()
	var out struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatalf("data 解码失败: %v", err)
	}
	dec, err := base64.StdEncoding.DecodeString(out.Value)
	if err != nil || len(dec) == 0 {
		t.Fatalf("value 应为合法 base64 PNG, err=%v len=%d", err, len(dec))
	}
	if _, err := png.Decode(bytes.NewReader(dec)); err != nil {
		t.Fatalf("value 解码应为 PNG: %v", err)
	}
}
