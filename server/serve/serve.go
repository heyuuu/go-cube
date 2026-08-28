package serve

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cube/version"
	"cube/web"
)

// 端口轮询参数。
const (
	probeInterval = 200 * time.Millisecond
	probeTimeout  = 10 * time.Second
)

// StatusInfo server 状态查询结果。
type StatusInfo struct {
	Running  bool   // 是否在跑（whoami 返回 app=="cube"）
	Version  string // whoami 返回的版本号（Running=false 时为空）
	Instance string // whoami 返回的实例标识（Running=false 或旧版 server 无此字段时为空）
}

// Status 通过 HTTP 探活：GET /api/system/whoami，验证返回 app==version.AppName。
//
// 只连端口不够——别的服务可能恰好监听同端口。验证 whoami 的 app 标记，
// 才能确认端口上确实是 cube server。
// 只区分「在跑/没在跑」：连不上、非 200、响应非法、身份不符一律降级为零值（未运行），
// 不返回 error——调用方只关心二元状态。
func Status(port int) StatusInfo {
	url := fmt.Sprintf("http://127.0.0.1:%d/api/system/whoami", port)
	resp, err := http.Get(url)
	if err != nil {
		return StatusInfo{} // 连不上 = 没在跑
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return StatusInfo{}
	}

	// whoami 经 ApiOutput envelope 包装：{ok, message, data:{app, version, instance}}
	var out struct {
		Ok   bool `json:"ok"`
		Data struct {
			App      string `json:"app"`
			Version  string `json:"version"`
			Instance string `json:"instance"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return StatusInfo{} // 响应非 cube 格式 = 不是 cube server
	}
	if out.Data.App != version.AppName {
		return StatusInfo{} // 端口上的服务不是 cube
	}
	return StatusInfo{Running: true, Version: out.Data.Version, Instance: out.Data.Instance}
}

// Stop 触发 server graceful shutdown：POST /api/system/shutdown（带 HMAC 鉴权），
// 轮询 whoami 确认「发起 Stop 时的那个实例」已下线。
//
// 下线按 whoami 的 instance 标识判定：探不到 cube、或 instance 已换人均算——
// launchctl 保活会在旧进程退出后立刻拉新进程占回端口，只看「端口上还有没有 cube」
// 会误判为没停掉。旧版 server 的 whoami 无 instance 字段（读作空串），被任何带
// instance 的新实例接管同样能判为已下线。
//
// 返回 stopped=true 表示发起时的实例确实关了，replaced=true 进一步表示端口已被
// 新实例接管（如 launchctl 自动拉起）。stopped=false 表示本来就没在跑。
// 不向 stdout 输出——面向用户的文案由调用方（cmd 层）决定。
func Stop(port int) (stopped bool, replaced bool, err error) {
	// 先探活，没在跑直接返回；同时记下当前实例标识
	st := Status(port)
	if !st.Running {
		return false, false, nil
	}
	old := st.Instance

	// 构造带 HMAC 鉴权的 shutdown 请求
	url := fmt.Sprintf("http://127.0.0.1:%d/api/system/shutdown", port)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return false, false, fmt.Errorf("构造 shutdown 请求失败: %w", err)
	}
	req.Header.Set(web.ShutdownTokenHeader, web.GenShutdownToken(time.Now()))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, false, fmt.Errorf("发送 shutdown 请求失败: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return false, false, fmt.Errorf("shutdown 请求被拒（status=%d，可能鉴权失败）", resp.StatusCode)
	}

	// 轮询确认旧实例下线
	replaced, err = waitOldDown(port, old, probeTimeout)
	if err != nil {
		return false, false, fmt.Errorf("shutdown 请求已发送但旧实例未下线: %w", err)
	}
	return true, replaced, nil
}

// waitOldDown 轮询 whoami 直到旧实例下线（探不到 cube 或 instance 换人）。
// 返回 replaced=true 表示端口已被新实例接管。
func waitOldDown(port int, oldInstance string, timeout time.Duration) (replaced bool, err error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st := Status(port)
		if !st.Running {
			return false, nil
		}
		if st.Instance != oldInstance {
			return true, nil
		}
		time.Sleep(probeInterval)
	}
	return false, fmt.Errorf("等待端口 %d 旧实例下线超时", port)
}
