package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// shutdownToken server 与 CLI 共享的鉴权密钥，编译进二进制。
//
// 用于 /api/system/shutdown 端点防 CSRF 式盲目调用：浏览器/外部网页拿不到这个
// token（它不在任何前端资源里暴露），也就构造不出有效的 HMAC 签名。本机其他
// 进程理论上能反编译二进制拿到 token，但那个威胁等级下它能直接 kill 进程，
// token 已不是瓶颈——本端点防的是「跨进程的盲目/重放调用」，不防「有 token 的本机进程」。
const shutdownToken = "cube-local-shutdown-v1"

// shutdownTokenWindow shutdown 请求的时间戳有效窗口（秒）。
// 本机调用毫秒级往返，5 秒窗口够宽裕又能防重放（抓包过 5 秒就作废）。
const shutdownTokenWindow = 5 * time.Second

// ShutdownTokenHeader HTTP header 名，值为 "<unix秒>.<hex(hmac-sha256(token, unix秒))>"。
const ShutdownTokenHeader = "X-Shutdown-Token"

// GenShutdownToken 生成 shutdown 鉴权 header 值：时间戳 + HMAC 签名。
//
// 供 CLI 端（serve.Stop）构造请求时调用。返回格式 "payload.sig"。
func GenShutdownToken(now time.Time) string {
	payload := strconv.FormatInt(now.Unix(), 10)
	sig := hmacHex([]byte(shutdownToken), []byte(payload))
	return payload + "." + sig
}

// VerifyShutdownToken 校验 shutdown 鉴权 header 值。
//
// 双重校验：
//  1. HMAC 比对（证明调用方持有 token）
//  2. 时间戳在窗口内（防重放）
//
// 任一失败返回 error。供 server 端 shutdown handler 调用。
func VerifyShutdownToken(headerVal string, now time.Time) error {
	payload, sig, ok := strings.Cut(headerVal, ".")
	if !ok {
		return fmt.Errorf("token 格式非法")
	}
	wantSig := hmacHex([]byte(shutdownToken), []byte(payload))
	if !hmac.Equal([]byte(sig), []byte(wantSig)) {
		return fmt.Errorf("token 签名不匹配")
	}
	ts, err := strconv.ParseInt(payload, 10, 64)
	if err != nil {
		return fmt.Errorf("token 时间戳非法: %w", err)
	}
	delta := now.Sub(time.Unix(ts, 0))
	if delta < 0 {
		delta = -delta
	}
	if delta > shutdownTokenWindow {
		return fmt.Errorf("token 已过期（窗口 %s）", shutdownTokenWindow)
	}
	return nil
}

// hmacHex 计算 HMAC-SHA256 的十六进制编码。
func hmacHex(key, msg []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(msg)
	return hex.EncodeToString(mac.Sum(nil))
}
