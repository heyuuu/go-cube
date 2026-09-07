package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/util/git"
	"cube/util/tui"
)

// newPushCmd 是 `cube push` 命令入口。
//
// 行为：query 定位已收录项目（规则同 info），以项目根为 git 仓库，
// 把多个分支 / tag 批量推送到多个 remote。remotes 与 refs 都通过 TUI
// 多选交互确认；当前分支默认勾选，全部 remote 默认勾选。最后展示推送
// 计划并二次确认后执行。
//
// 非 TTY 环境（脚本）下可改用 flag 显式指定：--remote（可多次）/ --ref（可多次），
// 此时跳过对应交互；--force 启用 --force-with-lease。
func newPushCmd(a *app.App) *cobra.Command {
	var (
		remotes []string
		refs    []string
		force   bool
		yes     bool
	)
	cmd := &cobra.Command{
		Use:   "push [query]",
		Short: "把选中的分支/tag 批量推送到多个 remote",
		Long: `把多个分支 / tag 批量推送到多个 remote。

query 定位目标项目（支持项目名或路径模糊搜索，规则同 info 命令），
不传时交互选择；以项目根目录作为 git 仓库执行后续操作。

remote 与 ref 默认通过 TUI 多选交互确认：remote 默认全选，
ref 默认勾选当前分支；执行前展示推送计划并二次确认。

非 TTY 环境（脚本）改用 flag 显式指定：--remote / --ref
（均可多次指定），跳过对应交互；--force 走 --force-with-lease，
--yes 跳过最终确认。单条 push 失败不中断，最后汇总结果。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// 定位项目：query 匹配（规则同 info），以项目根为仓库
			query := getArg(args, 0)
			proj, err := pickProject(a.ProjectService(), query)
			if err != nil {
				return err
			}
			repoPath := proj.Path()

			// 收集 remote / refs 候选
			repoRemotes, err := git.Remotes(repoPath)
			if err != nil {
				return fmt.Errorf("读取 remote 列表失败: %w", err)
			}
			if len(repoRemotes) == 0 {
				return errors.New("仓库未配置任何 remote，无可推送目标")
			}

			headRef := git.HeadRef(repoPath)
			repoRefs, err := git.Refs(repoPath)
			if err != nil {
				return fmt.Errorf("读取 ref 列表失败: %w", err)
			}

			// 选择 remote（flag 优先，否则 TUI 多选，默认全选）
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

			// 选择 ref（flag 优先，否则 TUI 多选，默认当前分支；标签带各 remote 的真实 ahead/behind）
			chosenRefs, err := pickRefs(repoPath, repoRefs, headRef, chosenRemotes, refs)
			if err != nil {
				if errors.Is(err, tui.ErrUserAborted) {
					return nil
				}
				return err
			}
			if len(chosenRefs) == 0 {
				return errors.New("未选择任何分支/tag")
			}

			// 展示推送计划 + 二次确认
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

			// 执行：remote 外层、ref 内层，逐条 push 并汇总结果
			return runPush(repoPath, chosenRemotes, chosenRefs, force)
		},
	}
	cmd.Flags().StringArrayVarP(&remotes, "remote", "r", nil, "目标 remote（可多次指定，默认交互多选）")
	cmd.Flags().StringArrayVarP(&refs, "ref", "b", nil, "待推送的 ref：分支名/tag 名（可多次指定；传 --tags 表示全部 tag；默认交互多选）")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "强制推送（使用 --force-with-lease）")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "跳过最终确认，直接推送")
	return cmd
}

// pickRemotes 决定要推送的 remote 集合：flag 显式指定优先，否则 TUI 多选（默认全选）。
func pickRemotes(all []git.Remote, flagRemotes []string) ([]git.Remote, error) {
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
		func(r git.Remote) string { return fmt.Sprintf("%s  (%s)", r.Name, r.Push) },
		all,
	)
}

// resolveRemotesByName 把 flag 传入的 remote 名解析成 git.Remote（找不到则报错）。
func resolveRemotesByName(all []git.Remote, names []string) ([]git.Remote, error) {
	byName := make(map[string]git.Remote, len(all))
	for _, r := range all {
		byName[r.Name] = r
	}
	var result []git.Remote
	for _, n := range names {
		r, ok := byName[n]
		if !ok {
			return nil, fmt.Errorf("未找到 remote: %s（可用: %s）", n, joinRemoteNames(all))
		}
		result = append(result, r)
	}
	return result, nil
}

// refAllTags 候选哨兵值：代表一次性推送全部本地 tag（git push --tags），非真实 git ref。
const refAllTags = "--tags"

// pickRefs 决定要推送的 ref 集合：flag 显式指定优先，否则 TUI 多选（默认当前分支）。
//   - 候选 = 本地分支 + 一条「全部 tags」合并项（tag 数量随时间线性增长，逐条列出
//     只会让候选列表越来越长，故合并成 refAllTags 一项整体推送）。
//   - 默认勾选当前分支（若存在）。
//   - 分支标签带相对各选中 remote 的真实 ahead/behind（先 fetch 刷新 remote-tracking，
//     单 remote 显示 "↑2 ↓0"，多 remote 逐个标注 "origin ↑2↓0"）；tag 无此语义。
//     fetch 失败（网络不通等）打警告降级为不显示，不阻断选择。
func pickRefs(repoPath string, refs *git.RefsResult, headRef string, remotes []git.Remote, flagRefs []string) ([]string, error) {
	if len(flagRefs) > 0 {
		return flagRefs, nil
	}
	if !tui.IsTTY() {
		return nil, errors.New("非交互环境(tty)下必须通过 --ref 指定分支/tag")
	}

	fetched := make(map[string]bool, len(remotes))
	for _, r := range remotes {
		fmt.Printf("正在 fetch %s ...", r.Name)
		if err := git.Fetch(repoPath, r.Name); err != nil {
			fmt.Printf(" 失败，ahead/behind 可能不准: %v\n", err)
		} else {
			fmt.Println(" 完成")
			fetched[r.Name] = true
		}
	}
	// fetch 后重读 refs，拿到最新 remote-tracking；远端没有同名分支的显示 ↑0 ↓0 会误导（像是已同步），跳过
	remoteBranches := map[string]map[string]bool{}
	if fresh, err := git.Refs(repoPath); err == nil {
		remoteBranches = buildRemoteBranchMap(fresh)
	}

	type refItem struct {
		label string
		ref   string
	}
	var items []refItem
	for _, ref := range refs.Locals {
		items = append(items, refItem{label: "branch: " + ref.ShortName + aheadBehindSuffix(repoPath, ref.ShortName, remotes, fetched, remoteBranches), ref: ref.Name})
	}
	if len(refs.Tags) > 0 {
		items = append(items, refItem{
			label: fmt.Sprintf("tags:   全部 %d 个", len(refs.Tags)),
			ref:   refAllTags,
		})
	}
	if len(items) == 0 {
		return nil, errors.New("仓库无任何本地分支或 tag")
	}

	// 默认勾选当前分支（匹配 ref == currentBranch）
	var defaults []refItem
	if headRef != "" {
		for _, it := range items {
			if it.ref == headRef {
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

// aheadBehindSuffix 生成分支标签的 ahead/behind 后缀（真实值，依赖调用方已 fetch）。
// 远端没有同名分支、或无任何 fetch 成功的 remote 时返回空串；单 remote 省略 remote 名，多 remote 逐个标注。
func aheadBehindSuffix(repoPath, branch string, remotes []git.Remote, fetched map[string]bool, remoteBranches map[string]map[string]bool) string {
	var parts []string
	for _, r := range remotes {
		if !fetched[r.Name] || !remoteBranches[branch][r.Name] {
			continue
		}
		ahead, behind, _ := git.AheadBehindRemote(repoPath, branch, r.Name, branch)
		if len(remotes) == 1 {
			parts = append(parts, fmt.Sprintf("  ↑%d ↓%d", ahead, behind))
		} else {
			parts = append(parts, fmt.Sprintf("  %s ↑%d↓%d", r.Name, ahead, behind))
		}
	}
	return strings.Join(parts, "")
}

// confirmPlan 打印推送计划表并要求二次确认。返回 (是否确认, 错误)。
func confirmPlan(repoPath string, remotes []git.Remote, refs []string, force bool) (bool, error) {
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
	return tui.ConfirmInlineDefault(fmt.Sprintf("确认推送到以上 %d 个 remote × %d 个 ref？", len(remotes), len(refs)), true)
}

// refDisplayName 返回 ref 在计划表中的展示名；哨兵 refAllTags 展示为更友好的说明。
func refDisplayName(ref string) string {
	if ref == refAllTags {
		return "全部本地 tag (--tags)"
	}
	return ref
}

// buildPlanRows 展开成 (remote × ref) 行用于表格展示。
func buildPlanRows(remotes []git.Remote, refs []string) [][]string {
	var rows [][]string
	for _, r := range remotes {
		for _, ref := range refs {
			rows = append(rows, []string{r.Name, r.Push, refDisplayName(ref)})
		}
	}
	return rows
}

// runPush 逐条执行 git push，并打印汇总。
//
// 单条 push 失败不中断后续（一个 remote 网络抖动不应阻断其它 remote），
// 全部跑完后若有失败则汇总返回一个 error。
func runPush(repoPath string, remotes []git.Remote, refs []string, force bool) error {
	total := len(remotes) * len(refs)
	failed := 0
	i := 0
	for _, r := range remotes {
		for _, ref := range refs {
			i++
			fmt.Printf("\n[%d/%d] git push %s %s%s\n", i, total, r.Name, refDisplayName(ref), forceFlagSuffix(force))
			var err error
			if ref == refAllTags {
				err = git.PushAllTags(repoPath, r.Name, force)
			} else {
				err = git.Push(repoPath, r.Name, ref, force)
			}
			if err != nil {
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
func joinRemoteNames(remotes []git.Remote) string {
	names := make([]string, len(remotes))
	for i, r := range remotes {
		names[i] = r.Name
	}
	return strings.Join(names, ", ")
}
