package web

import (
	"testing"
	"time"
)

func TestShutdownToken_RoundTrip(t *testing.T) {
	now := time.Unix(1700000000, 0)
	tok := GenShutdownToken(now)
	if err := VerifyShutdownToken(tok, now); err != nil {
		t.Errorf("合法 token 校验应通过，got err: %v", err)
	}
}

func TestShutdownToken_WithinWindow(t *testing.T) {
	gen := time.Unix(1700000000, 0)
	verify := gen.Add(4 * time.Second) // 窗口内
	tok := GenShutdownToken(gen)
	if err := VerifyShutdownToken(tok, verify); err != nil {
		t.Errorf("窗口内应通过（4s < 5s），got err: %v", err)
	}
}

func TestShutdownToken_Expired(t *testing.T) {
	gen := time.Unix(1700000000, 0)
	verify := gen.Add(6 * time.Second) // 超窗口
	tok := GenShutdownToken(gen)
	if err := VerifyShutdownToken(tok, verify); err == nil {
		t.Error("超窗口应拒绝（6s > 5s）")
	}
}

func TestShutdownToken_Future(t *testing.T) {
	gen := time.Unix(1700000000, 0)
	verify := gen.Add(-6 * time.Second) // "未来"请求（gen 在 verify 之后）
	tok := GenShutdownToken(gen)
	if err := VerifyShutdownToken(tok, verify); err == nil {
		t.Error("未来时间戳（超出反向窗口）应拒绝")
	}
}

func TestShutdownToken_BadFormat(t *testing.T) {
	cases := []string{
		"",
		"noseparator",
		"only.one.too.many",
		"1700000000", // 缺 sig
		".nosig",
		"noplayload.",
	}
	for _, c := range cases {
		if err := VerifyShutdownToken(c, time.Unix(1700000000, 0)); err == nil {
			t.Errorf("格式非法 %q 应报错", c)
		}
	}
}

func TestShutdownToken_BadSignature(t *testing.T) {
	// 正确 payload，错误 sig
	tok := "1700000000.deadbeef"
	if err := VerifyShutdownToken(tok, time.Unix(1700000000, 0)); err == nil {
		t.Error("错误签名应拒绝")
	}
}

func TestShutdownToken_BadTimestamp(t *testing.T) {
	// 用错误 payload 生成 sig，导致时间戳解析失败
	sig := hmacHex([]byte(shutdownToken), []byte("not-a-number"))
	tok := "not-a-number." + sig
	if err := VerifyShutdownToken(tok, time.Unix(1700000000, 0)); err == nil {
		t.Error("时间戳非数字应拒绝")
	}
}
