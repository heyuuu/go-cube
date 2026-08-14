package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"cube/app"
)

func newInfoCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info [query]",
		Short: "项目详情(支持项目名或项目路径模糊搜索)",
		Long: `显示单个项目的详情。

query 支持两种搜索模式：
  - 项目名称搜索：按关键词模糊匹配项目名称（默认）。
  - 项目路径搜索：当 query 以 '.'、'~' 或 '/' 开头时触发，
    搜索给定路径及其所有子目录中的项目。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := getArg(args, 0)

			// 匹配项目
			proj, err := pickProject(a.ProjectService(), query)
			if err != nil {
				return err
			}

			printInfoKV("project", proj.Name())
			printInfoKV("path", proj.Path())
			printInfoKV("group", orInfoDash(proj.Group()))
			printInfoKV("tags", orInfoDash(strings.Join(proj.Tags(), ", ")))

			// git 部分读缓存快照，不实时采集
			info, ok := a.ProjectService().GitInfo(proj.Path())
			if !ok {
				printInfoKV("git", "无缓存(可启动 cube server 自动采集)")
				return nil
			}
			printInfoKV("git-url", orInfoDash(info.RepoUrl))
			branch := info.CurrentBranch
			if branch == "" {
				branch = "HEAD(detached)" // CurrentBranch 为空即 detached HEAD
			}
			printInfoKV("branch", branch)
			if info.DefaultBranch != "" {
				// ahead/behind 是默认分支相对 origin 的差异
				printInfoKV("default", info.DefaultBranch+" "+formatAheadBehind(info.Ahead, info.Behind))
			}
			printInfoKV("dirty", boolToCn(info.Dirty))
			if info.WorktreeMain != "" {
				printInfoKV("worktree-main", info.WorktreeMain)
			}
			printInfoKV("branches", fmt.Sprintf("%d 个", len(info.Branches)))
			printInfoKV("snapshot", info.CollectedAt.Format("2006-01-02 15:04:05"))

			return nil
		},
	}
	return cmd
}

// printInfoKV 按 info 命令的对齐格式打印一行 key: value。
func printInfoKV(key, value string) {
	fmt.Printf("%-13s: %s\n", key, value)
}

// orInfoDash 空字符串显示为 "-"，避免输出空值造成阅读歧义。
func orInfoDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// boolToCn 布尔值转中文「是/否」。
func boolToCn(b bool) string {
	if b {
		return "是"
	}
	return "否"
}
