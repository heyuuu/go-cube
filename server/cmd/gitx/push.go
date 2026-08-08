package gitx

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"cube/cmd/util/easycobra"
	"cube/cmd/util/tui"
	"cube/util/git"
	"cube/util/gogit"
	"cube/util/pathkit"
)

// pushCmd 是 `cube gitx push` 命令入口。
//
// 行为：在指定仓库（默认从当前工作目录向上探测）里，把多个分支 / tag
// 批量推送到多个 remote。remotes 与 refs 都通过 TUI 多选交互确认；
// 当前分支默认勾选，全部 remote 默认勾选。最后展示推送计划并二次确认后执行。
//
// 非 TTY 环境（脚本）下可改用 flag 显式指定：--remote（可多次）/ --ref（可多次），
// 此时跳过对应交互；--force 启用 --force-with-lease。
var pushCmd = &easycobra.Command{
	Use:   "push [仓库路径]",
	Short: "把选中的分支/tag 批量推送到多个 remote",
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		var (
			remotes []string
			refs    []string
			force   bool
			yes     bool
		)
		cmd.Flags().StringArrayVarP(&remotes, "remote", "r", nil, "目标 remote（可多次指定，默认交互多选）")
		cmd.Flags().StringArrayVarP(&refs, "ref", "b", nil, "待推送的 ref：分支名/tag 名（可多次指定，默认交互多选）")
		cmd.Flags().BoolVarP(&force, "force", "f", false, "强制推送（使用 --force-with-lease）")
		cmd.Flags().BoolVarP(&yes, "yes", "y", false, "跳过最终确认，直接推送")

		return func(args []string) error {
			// 1. 解析仓库根：参数路径（可空）展开为绝对路径，再向上探测 .git
			var pathHint string
			if len(args) > 0 {
				pathHint = args[0]
			}
			start, err := pathkit.ResolvePath(pathHint)
			if err != nil {
				return err
			}
			repoPath, ok := git.FindGitRoot(start)
			if !ok {
				return fmt.Errorf("未找到 git 仓库（向上探测 .git 失败）: %s", start)
			}

			// 2. 收集 remote / refs 候选
			repoRemotes, err := gogit.Remotes(repoPath)
			if err != nil {
				return fmt.Errorf("读取 remote 列表失败: %w", err)
			}
			if len(repoRemotes) == 0 {
				return errors.New("仓库未配置任何 remote，无可推送目标")
			}

			localBranches, currentBranch, err := gogit.Branches(repoPath)
			if err != nil {
				return fmt.Errorf("读取分支列表失败: %w", err)
			}
			branches := filterLocalBranches(localBranches)
			tags, err := gogit.Tags(repoPath)
			if err != nil {
				return fmt.Errorf("读取 tag 列表失败: %w", err)
			}

			// 3. 选择 remote（flag 优先，否则 TUI 多选，默认全选）
			chosenRemotes, err := pickRemotes(repoRemotes, remotes)
			if err != nil {
				if errors.Is(err, tui.ErrUserAborted) {
					return nil
				}
				return err
			}
			if len(chosenRemotes) == 0 {
				return errors.New("未选择任何 remote")
			}

			// 4. 选择 ref（flag 优先，否则 TUI 多选，默认当前分支）
			chosenRefs, err := pickRefs(branches, tags, currentBranch, refs)
			if err != nil {
				if errors.Is(err, tui.ErrUserAborted) {
					return nil
				}
				return err
			}
			if len(chosenRefs) == 0 {
				return errors.New("未选择任何分支/tag")
			}

			// 5. 展示推送计划 + 二次确认
			if !yes {
				ok, err := confirmPlan(repoPath, chosenRemotes, chosenRefs, force)
				if err != nil {
					if errors.Is(err, tui.ErrUserAborted) {
						return nil
					}
					return err
				}
				if !ok {
					return nil
				}
			}

			// 6. 执行：remote 外层、ref 内层，逐条 push 并汇总结果
			return runPush(repoPath, chosenRemotes, chosenRefs, force)
		}
	},
}

// filterLocalBranches 从 gogit.Branches 的输出里筛出本地分支（去掉 origin/* 这种远程分支）。
func filterLocalBranches(all []string) []string {
	var local []string
	for _, b := range all {
		if strings.Contains(b, "/") {
			continue // origin/master 等远程分支
		}
		local = append(local, b)
	}
	return local
}

// pickRemotes 决定要推送的 remote 集合：flag 显式指定优先，否则 TUI 多选（默认全选）。
func pickRemotes(all []gogit.Remote, flagRemotes []string) ([]gogit.Remote, error) {
	if len(flagRemotes) > 0 {
		return resolveRemotesByName(all, flagRemotes)
	}
	if !tui.IsTTY() {
		return nil, errors.New("非交互环境(tty)下必须通过 --remote 指定 remote")
	}
	// 默认全选：把 all 整体作为 defaults
	return tui.MultiSelectItemWithDefaults(
		"选择目标 remote（空格勾选，回车确认）",
		all,
		func(r gogit.Remote) string { return fmt.Sprintf("%s  (%s)", r.Name, r.Push) },
		all,
	)
}

// resolveRemotesByName 把 flag 传入的 remote 名解析成 gogit.Remote（找不到则报错）。
func resolveRemotesByName(all []gogit.Remote, names []string) ([]gogit.Remote, error) {
	byName := make(map[string]gogit.Remote, len(all))
	for _, r := range all {
		byName[r.Name] = r
	}
	var result []gogit.Remote
	for _, n := range names {
		r, ok := byName[n]
		if !ok {
			return nil, fmt.Errorf("未找到 remote: %s（可用: %s）", n, joinRemoteNames(all))
		}
		result = append(result, r)
	}
	return result, nil
}

// pickRefs 决定要推送的 ref 集合：flag 显式指定优先，否则 TUI 多选（默认当前分支）。
//   - 候选 = 本地分支 + tag（前缀区分：分支裸名，tag 加 refs/tags/ 前缀推送更稳）。
//   - 默认勾选当前分支（若存在）。
func pickRefs(branches []string, tags []string, currentBranch string, flagRefs []string) ([]string, error) {
	if len(flagRefs) > 0 {
		return flagRefs, nil
	}
	if !tui.IsTTY() {
		return nil, errors.New("非交互环境(tty)下必须通过 --ref 指定分支/tag")
	}

	type refItem struct {
		label string
		ref   string
	}
	var items []refItem
	for _, b := range branches {
		items = append(items, refItem{label: "branch: " + b, ref: b})
	}
	for _, t := range tags {
		items = append(items, refItem{label: "tag:    " + t, ref: "refs/tags/" + t})
	}
	if len(items) == 0 {
		return nil, errors.New("仓库无任何本地分支或 tag")
	}

	// 默认勾选当前分支（匹配 ref == currentBranch）
	var defaults []refItem
	if currentBranch != "" {
		for _, it := range items {
			if it.ref == currentBranch {
				defaults = append(defaults, it)
				break
			}
		}
	}

	picked, err := tui.MultiSelectItemWithDefaults(
		"选择待推送的分支/tag（空格勾选，回车确认）",
		items,
		func(it refItem) string { return it.label },
		defaults,
	)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(picked))
	for i, it := range picked {
		result[i] = it.ref
	}
	return result, nil
}

// confirmPlan 打印推送计划表并要求二次确认。返回 (是否确认, 错误)。
func confirmPlan(repoPath string, remotes []gogit.Remote, refs []string, force bool) (bool, error) {
	fmt.Println()
	fmt.Printf("仓库: %s\n", repoPath)
	if force {
		fmt.Println("模式: 强制推送（--force-with-lease）")
	}
	fmt.Println("推送计划：")
	tui.PrintTable(
		[]string{"Remote", "URL", "Ref"},
		buildPlanRows(remotes, refs),
	)
	fmt.Println()
	return tui.ConfirmInline(fmt.Sprintf("确认推送到以上 %d 个 remote × %d 个 ref？", len(remotes), len(refs)))
}

// buildPlanRows 展开成 (remote × ref) 行用于表格展示。
func buildPlanRows(remotes []gogit.Remote, refs []string) [][]string {
	var rows [][]string
	for _, r := range remotes {
		for _, ref := range refs {
			rows = append(rows, []string{r.Name, r.Push, ref})
		}
	}
	return rows
}

// runPush 逐条执行 git push，并打印汇总。
//
// 单条 push 失败不中断后续（一个 remote 网络抖动不应阻断其它 remote），
// 全部跑完后若有失败则汇总返回一个 error。
func runPush(repoPath string, remotes []gogit.Remote, refs []string, force bool) error {
	total := len(remotes) * len(refs)
	failed := 0
	i := 0
	for _, r := range remotes {
		for _, ref := range refs {
			i++
			fmt.Printf("\n[%d/%d] git push %s %s%s\n", i, total, r.Name, ref, forceFlagSuffix(force))
			if err := git.Push(repoPath, r.Name, ref, force); err != nil {
				failed++
				fmt.Printf("  ✗ 失败: %v\n", err)
			}
		}
	}

	fmt.Println()
	fmt.Printf("完成：%d 条成功，%d 条失败。\n", total-failed, failed)
	if failed > 0 {
		return fmt.Errorf("有 %d 条 push 失败，请检查上方输出", failed)
	}
	return nil
}

// forceFlagSuffix 仅供日志展示用，返回 " --force-with-lease" 或空串。
func forceFlagSuffix(force bool) string {
	if force {
		return " --force-with-lease"
	}
	return ""
}

// joinRemoteNames 把 remote 名列表拼成逗号分隔串，用于错误提示。
func joinRemoteNames(remotes []gogit.Remote) string {
	names := make([]string, len(remotes))
	for i, r := range remotes {
		names[i] = r.Name
	}
	return strings.Join(names, ", ")
}
