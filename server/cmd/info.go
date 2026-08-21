package cmd

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"cube/app"
	"cube/util/git"
	"cube/util/tui"
)

// info 输出的样式组。键名青色与 PrintTable 表头同源（color 51），
// 次要信息灰色、状态色绿/黄，保持 CLI 整体视觉一致。
var (
	infoKeyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("51"))            // 键名
	infoDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))           // 次要信息（网页地址、绝对时间等）
	infoOkStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))            // 正常状态（未修改等）
	infoWarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))           // 需注意状态（有改动、无缓存等）
	infoBoldStyle    = lipgloss.NewStyle().Bold(true)                                  // 强调（项目名）
	infoSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")) // -v 小节标题（与表格边框同色）
)

// infoKeyWidth 是键名列宽度（含对齐），多行值的续行缩进也依赖它。
const infoKeyWidth = 13

func newInfoCmd(a *app.App) *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "info [query]",
		Short: "项目详情(支持项目名或项目路径模糊搜索)",
		Long: `显示单个项目的详情。

query 支持两种搜索模式：
  - 项目名称搜索：按关键词模糊匹配项目名称（默认）。
  - 项目路径搜索：当 query 以 '.'、'~' 或 '/' 开头时触发，
    搜索给定路径及其所有子目录中的项目。

-v 显示扩展信息（实时读仓库，不依赖缓存快照）：
  - 未提交的文件列表（类似 git status --short）；
  - 各本地分支与每个 remote 同名分支的 ahead/behind 宽表；
  - 按差距生成的 cube pull / cube push 建议命令。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := getArg(args, 0)

			// 匹配项目
			proj, err := pickProject(a.ProjectService(), query)
			if err != nil {
				return err
			}

			printInfoKV("project", infoBoldStyle.Render(proj.Name()))
			printInfoKV("path", proj.Path())
			printInfoKV("group", orInfoDash(proj.Group()))
			printInfoKV("tags", orInfoDash(strings.Join(proj.Tags(), ", ")))

			// git 缓存快照（branch/dirty 等基础字段）；remote 列表走本地实时读
			info, hasCache := a.ProjectService().GitInfo(proj.Path())
			cacheUrl := ""
			if hasCache {
				cacheUrl = info.RepoUrl
			}
			printInfoRemotes(proj.Path(), cacheUrl)

			if !hasCache {
				printInfoKV("git", infoWarnStyle.Render("无缓存(可启动 cube server 自动采集)"))
			} else {
				branch := info.CurrentBranch
				if branch == "" {
					branch = "HEAD(detached)" // CurrentBranch 为空即 detached HEAD
				}
				printInfoKV("branch", branch)
				if info.DefaultBranch != "" {
					// ahead/behind 是默认分支相对 origin 的差异
					printInfoKV("default", info.DefaultBranch+" "+formatAheadBehind(info.Ahead, info.Behind))
				}
				printInfoKV("dirty", boolToCnColored(info.Dirty))
				if info.WorktreeMain != "" {
					printInfoKV("worktree-main", info.WorktreeMain)
				}
				printInfoKV("branches", fmt.Sprintf("%d 个", len(info.Branches)))
				printInfoKV("snapshot", formatInfoSnapshot(info.CollectedAt))
			}

			if verbose {
				printInfoVerbose(proj.Path())
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "显示扩展信息：未提交文件、各分支与 remote 的差距、pull/push 建议")
	return cmd
}

// printInfoKV 按 info 命令的对齐格式打印一行 key: value。
// 键名用 lipgloss Width 补齐到 13 列（比 %-13s 的好处是按显示宽度计算，
// 键名带 ANSI 色码时不会错位）；整行走 tui.Print，非 TTY / NO_COLOR 自动降级剥色。
func printInfoKV(key, value string) {
	tui.Print(infoKeyStyle.Width(infoKeyWidth).Render(key) + ": " + value + "\n")
}

// printInfoSection 打印 -v 的小节标题（前置空行 + 加粗着色标题）。
func printInfoSection(title string) {
	tui.Print("\n" + infoSectionStyle.Render(title) + "\n")
}

// printInfoRemotes 打印 git-url 行，展示全部 remote：
//   - 单 remote：维持「url (网页地址)」形态（与历史输出一致）；
//   - 多 remote：每 remote 一行「名字  url (网页地址)」，名字列对齐，续行缩进对齐值列。
//
// remote 列表实时读本地 .git/config（毫秒级、不联网）；读不到时回退缓存快照里的
// origin 地址，保持仓库已不可读场景下的展示能力。
func printInfoRemotes(repoPath string, cacheUrl string) {
	remotes, _ := git.Remotes(repoPath)
	if len(remotes) == 0 && cacheUrl != "" {
		remotes = []git.Remote{{Name: "origin", Fetch: cacheUrl, Push: cacheUrl}}
	}

	if len(remotes) <= 1 {
		url := ""
		if len(remotes) == 1 {
			url = remotes[0].Fetch
		}
		printInfoKV("git-url", formatInfoGitUrl(url))
		return
	}

	lines := buildInfoRemoteLines(remotes)
	indent := strings.Repeat(" ", infoKeyWidth+2) // 键列 + ": " 的宽度，续行与值列对齐
	value := lines[0]
	for _, l := range lines[1:] {
		value += "\n" + indent + l
	}
	printInfoKV("git-url", value)
}

// buildInfoRemoteLines 构造多 remote 场景下每行「名字  url (网页地址)」，名字列按最长名对齐。
func buildInfoRemoteLines(remotes []git.Remote) []string {
	nameWidth := 0
	for _, r := range remotes {
		if len(r.Name) > nameWidth {
			nameWidth = len(r.Name)
		}
	}
	lines := make([]string, len(remotes))
	for i, r := range remotes {
		lines[i] = fmt.Sprintf("%-*s", nameWidth, r.Name) + "  " + formatInfoGitUrl(r.Fetch)
	}
	return lines
}

// formatInfoGitUrl 渲染单个仓库地址：能识别 host 时在括号里附上网页地址。
// 括号部分用灰色降为次要信息，视觉上不与仓库地址抢焦点。
func formatInfoGitUrl(repoUrl string) string {
	s := orInfoDash(repoUrl)
	if u, err := git.ParseRepoUrl(repoUrl); err == nil {
		if web := u.WebUrl(); web != "" {
			s += " " + infoDimStyle.Render("("+web+")")
		}
	}
	return s
}

// formatInfoSnapshot 渲染 snapshot 值：相对时间为主，绝对时间放括号里备查。
func formatInfoSnapshot(t time.Time) string {
	return prettyTime(t) + " " + infoDimStyle.Render("("+t.Format("2006-01-02 15:04:05")+")")
}

// printInfoVerbose 打印 -v 的三段扩展：未提交文件 / 分支同步状态 / pull·push 建议。
// 数据实时读本地仓库（remote 跟踪分支口径，不联网），不依赖缓存快照；
// 读失败按 info 的容错基调降级为警告行，不中断整体输出。
func printInfoVerbose(repoPath string) {
	// ── 未提交改动：工作区干净时整段不显示 ──
	st, err := git.LoadRepoStatus(repoPath)
	if err != nil {
		printInfoSection("未提交改动")
		tui.Print(infoWarnStyle.Render(fmt.Sprintf("读取工作区状态失败: %v", err)) + "\n")
	} else if len(st.Files) > 0 {
		printInfoSection("未提交改动")
		for _, f := range st.Files {
			tui.Print(formatInfoFileLine(f) + "\n")
		}
	}

	// ── 分支同步状态 + 同步建议：无 remote 时提示后结束 ──
	remotes, _ := git.Remotes(repoPath)
	if len(remotes) == 0 {
		printInfoSection("分支同步状态")
		tui.Print("仓库未配置任何 remote\n")
		return
	}

	currentBranch := git.CurrentBranch(repoPath)
	repoRefs, err := git.Refs(repoPath)
	if err != nil {
		printInfoSection("分支同步状态")
		tui.Print(infoWarnStyle.Render(fmt.Sprintf("读取 ref 列表失败: %v", err)) + "\n")
		return
	}

	remoteHasBranch := buildRemoteBranchMap(repoRefs)
	branches := pickSharedBranches(repoRefs, remoteHasBranch)
	diffs := collectBranchRemoteDiffs(repoPath, branches, remotes, remoteHasBranch)

	printInfoSection("分支同步状态")
	if len(branches) == 0 {
		tui.Print("没有本地与任一 remote 同名的分支，无可对比项。\n")
	} else {
		tui.PrintTable(buildStatusHeaders(remotes), buildStatusRowsFromDiffs(diffs, branches, remotes, currentBranch))
	}

	printInfoSection("同步建议")
	suggestions := buildSyncSuggestions(diffs, repoPath)
	if len(suggestions) == 0 {
		tui.Print("所有分支与远端一致，无需 pull/push。\n")
		return
	}
	for _, s := range suggestions {
		if s.Command == "" {
			// 分叉的分支没有安全的自动同步命令，仅提示
			tui.Print(infoWarnStyle.Render("# "+s.Note) + "\n")
		} else {
			tui.Print(s.Command + "  " + infoDimStyle.Render("← "+s.Note) + "\n")
		}
	}
}

// formatInfoFileLine 渲染一行未提交文件状态：XY 码 + 路径。
// 有实际改动（M/A/D/R/U）黄色提示；未跟踪（??）灰色弱化。
func formatInfoFileLine(f git.FileStatus) string {
	if strings.Contains(f.Code, "?") {
		return infoDimStyle.Render(f.Code) + " " + f.Path
	}
	return infoWarnStyle.Render(f.Code) + " " + f.Path
}

// syncSuggestion 一条同步建议：可复制的 cube 命令 + 差异说明。
// Command 为空串表示该对没有安全的自动同步方式（分叉），仅输出 Note。
type syncSuggestion struct {
	Command string
	Note    string
}

// buildSyncSuggestions 由 (分支×remote) 差距生成 pull/push 建议：
//   - 仅落后 → cube pull（远端有更新，快进可成功）
//   - 仅领先 → cube push（本地有更新）
//   - 分叉   → 不给命令（快进/普通推送都会被拒），提示手动处理
//
// 输出顺序：先 pull（先更新本地）再 push，分叉提示排最后。
func buildSyncSuggestions(diffs []branchRemoteDiff, repoPath string) []syncSuggestion {
	var out []syncSuggestion
	for _, d := range diffs {
		if d.Behind > 0 && d.Ahead == 0 {
			out = append(out, syncSuggestion{
				Command: fmt.Sprintf("cube pull %s -r %s -b %s", repoPath, d.Remote, d.Branch),
				Note:    fmt.Sprintf("远端领先 %d", d.Behind),
			})
		}
	}
	for _, d := range diffs {
		if d.Ahead > 0 && d.Behind == 0 {
			out = append(out, syncSuggestion{
				Command: fmt.Sprintf("cube push %s -r %s -b %s", repoPath, d.Remote, d.Branch),
				Note:    fmt.Sprintf("本地领先 %d", d.Ahead),
			})
		}
	}
	for _, d := range diffs {
		if d.Ahead > 0 && d.Behind > 0 {
			out = append(out, syncSuggestion{
				Note: fmt.Sprintf("%s 与 %s 分叉（本地领先 %d / 落后 %d），需手动 rebase/merge 后再同步",
					d.Branch, d.Remote, d.Ahead, d.Behind),
			})
		}
	}
	return out
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

// boolToCnColored 布尔值转中文「是/否」并着色：是=黄（需注意有改动），否=绿（干净）。
func boolToCnColored(b bool) string {
	if b {
		return infoWarnStyle.Render(boolToCn(b))
	}
	return infoOkStyle.Render(boolToCn(b))
}
