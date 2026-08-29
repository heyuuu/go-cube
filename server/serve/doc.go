// Package serve 通过 HTTP API 管理 server 进程的生命周期（探活、关停）。
//
//   - Status：GET /api/system/whoami 探活并验证身份（app=="cube"，返回实例标识）；
//   - Stop：POST /api/system/shutdown 触发服务自己 graceful shutdown（带 HMAC 鉴权），
//     按 whoami 的 instance 确认发起时的旧实例下线（探不到或换人均算）。
//
// 不提供后台 fork 启动：常驻由系统级保活承担（prod launchd / dev air，1036）。
package serve
