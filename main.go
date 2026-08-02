package main

import (
	"github.com/heyuuu/cube/cmd"
	"github.com/heyuuu/cube/web"
)

func main() {
	web.SetUIAssets(UIFS) // 装配层注入：把 embed 的前端资源交给 web 包消费
	cmd.Execute()
}
