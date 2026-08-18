package git

import (
	"fmt"
	"strconv"
	"strings"
)

// Commit commit 图的单条提交（提案 1011）。拓扑连线由前端根据 Parents 渲染，后端不预计算图。
type CommitEntry struct {
	Sha       string   `json:"sha"`       // 完整 sha
	ShortSha  string   `json:"shortSha"`  // 短 sha
	Parents   []string `json:"parents"`   // 父提交 sha 列表（合并提交有多个）
	Author    string   `json:"author"`    // 作者名
	Timestamp int64    `json:"timestamp"` // 提交时间（unix 秒）
	Refs      []string `json:"refs"`      // 挂在该提交上的 ref（分支/tag/HEAD），来自 %D 装饰
	Subject   string   `json:"subject"`   // 提交标题首行
}

// CommitsPage 按 ref（空 = 当前 HEAD）拉取一页 commit，skip/limit 分页。
// all=true 时带 --all（全部本地 ref），否则只走 ref 的单线历史。
// 分页稳定性依赖 git 对同一 ref 集合输出顺序确定（topo-order），翻页期间仓库有新提交
// 时可能出现边界重复，由前端按 sha 去重。
func CommitsPage(dir string, all bool, ref string, skip, limit int) ([]CommitEntry, error) {
	args := []string{"log", "--topo-order", "--date-order"}
	if all {
		args = append(args, "--all")
	}
	if ref != "" {
		args = append(args, ref)
	}
	args = append(args,
		"--skip="+strconv.Itoa(skip),
		"--max-count="+strconv.Itoa(limit),
		"--pretty=format:%H%x1f%h%x1f%P%x1f%an%x1f%at%x1f%D%x1f%s",
	)
	out, err := runOut(dir, args...)
	if err != nil {
		return nil, fmt.Errorf("git log 执行失败: %w", err)
	}
	return parseLogFields(out), nil
}

// parseLogFields 解析 \x1f 分隔的结构化 log 输出（每行一条 commit，无行尾分隔符）。
func parseLogFields(out string) []CommitEntry {
	var commits []CommitEntry
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\x1f")
		if len(f) < 7 {
			continue
		}
		ts, _ := strconv.ParseInt(f[4], 10, 64)
		commits = append(commits, CommitEntry{
			Sha:       f[0],
			ShortSha:  f[1],
			Parents:   splitNonEmpty(f[2], " "),
			Author:    f[3],
			Timestamp: ts,
			Refs:      parseDecorations(f[5]),
			Subject:   f[6],
		})
	}
	return commits
}

// parseDecorations 解析 %D 装饰串，如 "HEAD -> main, origin/main, tag: v1"。
// 保留原名（含 tag: 前缀归一为纯名），箭头取指向端。
func parseDecorations(d string) []string {
	if d == "" {
		return nil
	}
	var refs []string
	for _, part := range strings.Split(d, ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		if i := strings.Index(name, "->"); i >= 0 {
			name = strings.TrimSpace(name[i+2:])
		}
		name = strings.TrimPrefix(name, "tag: ")
		if name != "" {
			refs = append(refs, name)
		}
	}
	return refs
}

func splitNonEmpty(s, sep string) []string {
	var result []string
	for _, p := range strings.Split(s, sep) {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
