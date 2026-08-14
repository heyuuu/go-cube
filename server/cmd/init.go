package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/opener"
	"cube/util/git"
	"cube/util/pathkit"
	"cube/util/tui"
)

// cmd `cube init`
func newInitCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <path>",
		Short: "在指定目录初始化一个项目(本质是初始化 git 仓库)",
		Long: `在指定目录初始化一个新项目，本质是初始化 git 仓库。

目录必须能被 scan 规则收录为新项目，否则初始化出来的目录 cube 无法识别。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawPath := args[0]

			// 解析为绝对路径
			absPath, err := pathkit.AbsPath(rawPath)
			if err != nil {
				return fmt.Errorf("解析路径失败: %w", err)
			}

			// 检查：从当前目录向上查找，若任意层级已存在 .git 则报错退出
			if existingGit, ok := git.FindGitRoot(absPath); ok {
				return fmt.Errorf("目录 %s 已处于 git 仓库中 (.git 位于 %s)，无法重复初始化",
					pathkit.PrettyPath(absPath), pathkit.PrettyPath(existingGit))
			}

			// 检查：路径能被 scan 规则收录为新项目（init 出一个 cube 看不见的目录没有意义）
			_, projName, ok := a.ProjectService().MatchScanRule(absPath)
			if !ok {
				return fmt.Errorf("路径 %s 无法被 scan 收录为项目：需位于某条 scan 规则目录的 maxDepth 层级内，且各级目录名不以 . 或 _ 开头",
					pathkit.PrettyPath(absPath))
			}

			// 已存在时必须是目录；不存在走后面的创建流程
			info, err := os.Stat(absPath)
			if err == nil { // 路径存在时，判断是否为目录
				if !info.IsDir() {
					return fmt.Errorf("指定路径不是目录: %s", absPath)
				}
			} else if os.IsNotExist(err) { // 路径不存在时，询问是否创建
				err = confirmForCreateDir(absPath)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("目录不可访问: %s", absPath)
			}

			// 执行 git init
			fmt.Printf("> 在 %s 执行 git init\n", pathkit.PrettyPath(absPath))
			if err = git.Init(absPath); err != nil {
				return fmt.Errorf("git init 失败: %w", err)
			}

			fmt.Printf("\n> 项目初始化完成: %s\n", pathkit.PrettyPath(absPath))
			fmt.Printf("> 后续可用 cube open %s 打开项目\n", projName)

			// TTY 环境询问是否直接打开项目。不走项目搜索——新项目要下次扫描才会出现在列表里
			if !tui.IsTTY() {
				return nil
			}
			open, err := tui.Confirm("是否直接打开项目？")
			if err != nil {
				if errors.Is(err, tui.ErrNotTTY) {
					return nil // 非交互环境跳过打开询问；init 本身已完成，不报为失败
				}
				return err
			}
			if !open {
				return nil
			}

			// 打开逻辑同 cube open：按 open-dir role 挑选 opener 后打开目录
			openApp, err := pickOpener(a.OpenerService(), opener.RoleOpenDir, "")
			if err != nil {
				return err
			}
			if err = openApp.Open(absPath); err != nil {
				return fmt.Errorf("打开失败: %w", err)
			}
			return nil
		},
	}
	return cmd
}

func confirmForCreateDir(dir string) error {
	create, err := tui.Confirm(fmt.Sprintf("目录 %s 不存在，是否创建？", pathkit.PrettyPath(dir)))
	if err != nil {
		return fmt.Errorf("询问是否创建目录失败: %w", err)
	}
	if !create {
		return errors.New("用户取消创建目录，初始化终止")
	}
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	fmt.Printf("> 已创建目录 %s\n", pathkit.PrettyPath(dir))
	return nil
}
