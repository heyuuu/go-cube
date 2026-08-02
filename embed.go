package main

import "embed"

// UIFS 前端静态资源（ui/ 目录），由 main 在启动时注入给 web 包消费。
// 放在 main 包是因为 //go:embed 不允许引用上级目录（不能写 ../ui），
// 而仓库根目录属于 main 包，可合法 embed 同级的 ui/。
// 详见 docs/design/v3-frontend.md「资源接入」一节。
//
//go:embed ui
var UIFS embed.FS
