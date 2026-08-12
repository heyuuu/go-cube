// Package serve 通过 HTTP API 管理 server 进程的生命周期（探活、关停、后台 fork）。
//
//   - Status：GET /api/system/whoami 探活并验证身份（app=="cube"）；
//   - Stop：POST /api/system/shutdown 触发服务自己 graceful shutdown（带 HMAC 鉴权）；
//   - Fork：start --detach 时 fork 一个脱离终端的后台子进程跑前台模式。
package serve
