package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/util/git"
	"cube/util/slicekit"
	"cube/util/tui"
)

// newPullCmd 是 `cube pull` 命令入口（push 的反向：从单个 remote 批量更新本地分支）。
//
// 行为：query 定位已收录项目（规则同 push），选择一个来源 remote
// （仅 1 个 remote 时自动选中，多个时交互单选），多选分支后逐条快进更新：
//   - 当前分支：git pull --ff-only <remote> <branch>
//   - 非当前分支：git fetch <remote> <branch>:<branch>（git 不允许 fetch 更新当前分支）
//
// 仅快进：本地与远端分叉的分支失败跳过，绝不产生 merge commit。
// 单条失败不中断（同 push），最后汇总结果。
func newPullCmd(a *app.App) *cobra.Command {
	var (
		remote string
		refs   []string
		yes    bool
	)
	cmd := &cobra.Command{
		Use:   "pull [query]",
		Short: "从选中的 remote 批量快进更新本地分支",
		Long: `从单个 remote 批量拉取更新选中的分支（push 的反向命令）。

query 定位目标项目（支持项目名或路径模糊搜索，规则同 info 命令），
不传时交互选择；以项目根目录作为 git 仓库执行后续操作。

remote 选择：仅 1 个 remote 时自动选中；多个 remote 时交互单选。
分支默认通过 TUI 多选交互确认：候选为与该 remote 同名的本地分支，
默认勾选远端领先的分支（都不落后时默认当前分支）。

更新方式为仅快进：
  - 当前分支：git pull --ff-only <remote> <branch>
  - 非当前分支：git fetch <remote> <branch>:<branch>
本地与远端分叉的分支会失败跳过，不产生 merge commit。

分支列表里的领先/落后数基于本地记录的 remote 跟踪分支（不联网），
实际拉取时以远端最新状态为准。

非 TTY 环境（脚本）改用 flag 显式指定：--remote / --ref（可多次），
跳过对应交互；--yes 跳过最终确认。单条失败不中断，最后汇总结果。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. 定位项目：query 匹配（规则同 push），以项目根为仓库
			query := getArg(args, 0)
			proj, err := pickProject(a.ProjectService(), query)
			if err != nil {
				return err
			}
			repoPath := proj.Path()

			// 2. 收集 remote 列表
			repoRemotes, err := git.Remotes(repoPath)
			if err != nil {
				return fmt.Errorf("读取 remote 列表失败: %w", err)
			}
			if len(repoRemotes) == 0 {
				return errors.New("仓库未配置任何 remote，无可拉取来源")
			}

			// 3. 单选来源 remote（flag 优先 → 单 remote 自动 → 多 remote 手选）
			chosenRemote, err := pickPullRemote(repoRemotes, remote)
			if err != nil {
				if errors.Is(err, tui.ErrUserAborted) {
					return nil
				}
				return err
			}

			// 4. 候选分支：与该 remote 同名的本地分支及各自领先/落后数
			candidates, currentBranch, err := pullCandidates(repoPath, chosenRemote.Name)
			if err != nil {
				return err
			}
			if len(candidates) == 0 {
				return fmt.Errorf("remote %s 上没有与本地同名的分支，无可拉取项", chosenRemote.Name)
			}

			// 5. 多选分支（flag 优先，默认勾选远端领先的分支）
			chosenBranches, err := pickPullBranches(candidates, currentBranch, refs)
			if err != nil {
				if errors.Is(err, tui.ErrUserAborted) {
					return nil
				}
				return err
			}
			if len(chosenBranches) == 0 {
				return errors.New("未选择任何分支")
			}

			// 6. 展示拉取计划 + 二次确认
			if !yes {
				ok, err := confirmPullPlan(repoPath, chosenRemote, chosenBranches, currentBranch)
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

			// 7. 执行：逐条快进更新并汇总结果
			return runPull(repoPath, chosenRemote.Name, chosenBranches, currentBranch)
		},
	}
	cmd.Flags().StringVarP(&remote, "remote", "r", "", "拉取来源 remote（多个 remote 时必选，仅 1 个可省略）")
	cmd.Flags().StringArrayVarP(&refs, "ref", "b", nil, "待拉取的分支名（可多次指定，默认交互多选）")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "跳过最终确认，直接拉取")
	return cmd
}

// branchDiff 描述一个本地分支相对某 remote 同名分支的领先/落后 commit 数。
type branchDiff struct {
	Branch string
	Ahead  int // 本地领先
	Behind int // 本地落后（远端有更新）
}

// pullCandidates 计算与 remoteName 同名的本地分支列表及各自 ahead/behind。
// 数据基于本地记录的 remote 跟踪分支（不联网），实际拉取时以远端最新状态为准。
func pullCandidates(repoPath string, remoteName string) ([]branchDiff, string, error) {
	repoRefs, err := git.Refs(repoPath)
	if err != nil {
		return nil, "", fmt.Errorf("读取 ref 列表失败: %w", err)
	}
	currentBranch := git.CurrentBranch(repoPath)

	// 候选 = 与指定 remote 同名的本地分支（其它 remote 的同名分支不参与）
	remoteHasBranch := buildRemoteBranchMap(repoRefs)
	var diffs []branchDiff
	for _, ref := range repoRefs.Locals {
		b := ref.Branch
		if !remoteHasBranch[b][remoteName] {
			continue
		}
		ahead, behind, _ := git.AheadBehindRemote(repoPath, b, remoteName, b)
		diffs = append(diffs, branchDiff{Branch: b, Ahead: ahead, Behind: behind})
	}
	return diffs, currentBranch, nil
}

// pickPullRemote 决定拉取来源 remote：flag 指定优先；仅 1 个自动选中；
// 多个时交互单选（pull 一次只面对一个 remote，与 push 的多选不同）。
func pickPullRemote(all []git.Remote, flagRemote string) (git.Remote, error) {
	if flagRemote != "" {
		rs, err := resolveRemotesByName(all, []string{flagRemote})
		if err != nil {
			return git.Remote{}, err
		}
		return rs[0], nil
	}
	if len(all) == 1 {
		return all[0], nil
	}
	if !tui.IsTTY() {
		return git.Remote{}, fmt.Errorf("非交互环境(tty)下必须通过 --remote 指定 remote（可选: %s）", joinRemoteNames(all))
	}
	return tui.SelectItem("选择拉取来源 remote", all, func(r git.Remote) string {
		return fmt.Sprintf("%s  (%s)", r.Name, r.Fetch)
	})
}

// pickPullBranches 决定待拉取的分支集合：flag 显式指定优先（校验属于候选），
// 否则 TUI 多选，默认勾选 pullDefaultBranches 的结果。
func pickPullBranches(candidates []branchDiff, currentBranch string, flagRefs []string) ([]string, error) {
	if len(flagRefs) > 0 {
		valid := make(map[string]bool, len(candidates))
		for _, d := range candidates {
			valid[d.Branch] = true
		}
		for _, r := range flagRefs {
			if !valid[r] {
				return nil, fmt.Errorf("分支 %s 不在 remote 的同名分支候选中", r)
			}
		}
		return flagRefs, nil
	}
	if !tui.IsTTY() {
		return nil, errors.New("非交互环境(tty)下必须通过 --ref 指定分支")
	}

	picked, err := tui.MultiSelectItemWithDefaults(
		"选择待拉取的分支（空格勾选，回车确认）",
		candidates,
		func(d branchDiff) string { return pullBranchLabel(d, currentBranch) },
		pullDefaultBranches(candidates, currentBranch),
	)
	if err != nil {
		return nil, err
	}
	return slicekit.Map(picked, func(d branchDiff) string { return d.Branch }), nil
}

// pullDefaultBranches 计算多选列表的默认勾选项：远端领先的分支；
// 都不落后时退回当前分支（拉取最常见的就是更新当前分支）。
func pullDefaultBranches(candidates []branchDiff, currentBranch string) []branchDiff {
	var defaults []branchDiff
	for _, d := range candidates {
		if d.Behind > 0 {
			defaults = append(defaults, d)
		}
	}
	if len(defaults) == 0 && currentBranch != "" {
		for _, d := range candidates {
			if d.Branch == currentBranch {
				defaults = append(defaults, d)
				break
			}
		}
	}
	return defaults
}

// pullBranchLabel 构造分支在多选列表里的展示文案：分支名 + 当前分支标记 + 同步状态。
func pullBranchLabel(d branchDiff, currentBranch string) string {
	name := d.Branch
	if d.Branch == currentBranch {
		name = "* " + name
	}
	var state string
	switch {
	case d.Ahead > 0 && d.Behind > 0:
		state = fmt.Sprintf("分叉：本地领先 %d / 落后 %d", d.Ahead, d.Behind)
	case d.Behind > 0:
		state = fmt.Sprintf("落后 %d", d.Behind)
	case d.Ahead > 0:
		state = fmt.Sprintf("本地领先 %d", d.Ahead)
	default:
		state = "已同步"
	}
	return fmt.Sprintf("branch: %s  (%s)", name, state)
}

// confirmPullPlan 打印拉取计划表并要求二次确认。返回 (是否确认, 错误)。
func confirmPullPlan(repoPath string, remote git.Remote, branches []string, currentBranch string) (bool, error) {
	fmt.Println()
	fmt.Printf("仓库: %s\n", repoPath)
	fmt.Printf("Remote: %s  (%s)\n", remote.Name, remote.Fetch)
	fmt.Println("拉取计划（仅快进，分叉的分支会失败跳过）：")
	rows := make([][]string, 0, len(branches))
	for _, b := range branches {
		method := "fetch 快进"
		if b == currentBranch {
			method = "pull --ff-only（当前分支）"
		}
		rows = append(rows, []string{b, method})
	}
	tui.PrintTable([]string{"Ref", "方式"}, rows)
	fmt.Println()
	return tui.ConfirmInline(fmt.Sprintf("确认从 %s 拉取以上 %d 个分支？", remote.Name, len(branches)))
}

// runPull 逐条执行拉取，best-effort：单条失败不中断后续（同 push），最后汇总。
func runPull(repoPath string, remoteName string, branches []string, currentBranch string) error {
	failed := 0
	for i, b := range branches {
		fmt.Printf("\n[%d/%d] ", i+1, len(branches))
		var err error
		if b == currentBranch {
			fmt.Printf("git pull --ff-only %s %s\n", remoteName, b)
			err = git.Pull(repoPath, remoteName, b)
		} else {
			fmt.Printf("git fetch %s %s:%s\n", remoteName, b, b)
			err = git.FetchIntoBranch(repoPath, remoteName, b)
		}
		if err != nil {
			failed++
			fmt.Printf("  ✗ 失败: %v\n", err)
		}
	}

	fmt.Println()
	fmt.Printf("完成：%d 条成功，%d 条失败。\n", len(branches)-failed, failed)
	if failed > 0 {
		return fmt.Errorf("有 %d 条拉取失败，请检查上方输出", failed)
	}
	return nil
}
