package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/serve"
	"cube/util/pathkit"
)

// newMdCmd `cube md <path>` —— 以 Web 方式打开 markdown 文件。
//
// server 不在跑时报错退出（不做 lazy 拉起，遵循「显式 start」的 server 管理模式）；
// 页面与渲染归前端工程（/md?path=<abs>），本命令只负责拼 URL 并开浏览器。
func newMdCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "md <path>",
		Short: "以 Web 方式打开 markdown 文件",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMd(a, args[0])
		},
	}
	return cmd
}

func runMd(a *app.App, rawPath string) error {
	// 路径解析：支持 ~ 与相对路径，统一转绝对路径（web API 只收绝对路径）。
	// pathkit 只认显式相对（./ ../），裸文件名在此补 ./ 前缀（cwd 解析属出口层职责）
	if rawPath[0] != '/' && rawPath[0] != '~' && rawPath[0] != '.' {
		rawPath = "./" + rawPath
	}
	absPath, err := pathkit.AbsPath(rawPath)
	if err != nil {
		return fmt.Errorf("解析路径失败: %w", err)
	}
	// 目录也放行：/md 页对目录展示左侧文件树（无 md 的目录显示空态）
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("路径不存在: %s", absPath)
	}

	// 前置：server 必须在跑（渲染页面由 server 的前端承载）
	if st, _ := serve.Status(a.Server().Port()); !st.Running {
		return errors.New("server 未运行，请先执行: cube server start")
	}

	pageURL := fmt.Sprintf("%smd?path=%s", a.Server().ServerURL(), url.QueryEscape(absPath))
	if err := exec.Command("open", pageURL).Run(); err != nil {
		// 打开浏览器失败不阻断：把 URL 打出来让用户手动访问
		fmt.Printf("自动打开浏览器失败，请手动访问：%s\n", pageURL)
	} else {
		fmt.Printf("%s\n", pageURL)
	}
	return nil
}
