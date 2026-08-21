package git

// remote 查询：远端配置（remote 列表与地址）。与 refs.go 的分界——本文件回答
// 「仓库配了哪些 remote」，refs.go 回答「有哪些 ref」（含 refs/remotes/* 跟踪 ref）。
// 执行核与错误约定见 run.go。

import (
	"sort"
	"strings"
)

// Remote 描述一个 remote：名字 + 抓取/推送地址（取各自的第一条）。
type Remote struct {
	Name  string
	Fetch string
	Push  string
}

// Remotes 返回 path 处仓库的全部 remote（按名字排序）。
// Fetch / Push 取各自的第一条地址；配置了独立 pushurl 的 remote 两者不同。
// 非仓库目录或无任何 remote 时返回 (nil, nil)，不视为错误。
func Remotes(path string) ([]Remote, error) {
	if !isGitRepo(path) {
		return nil, nil
	}
	out, err := runOut(path, "remote", "-v")
	if err != nil {
		return nil, err
	}
	return parseRemotesVerbose(out), nil
}

// parseRemotesVerbose 解析 `git remote -v` 输出，每行形如（name 与 url 间是 TAB）：
//
//	origin	git@github.com:a/b.git (fetch)
//	origin	git@github.com:a/b.git (push)
//
// 同一 remote 的 fetch/push 行合并为一条，同名多行取第一条（多 URL remote 属罕见配置）。
func parseRemotesVerbose(out string) []Remote {
	index := make(map[string]*Remote)
	for _, line := range strings.Split(out, "\n") {
		name, rest, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		url, field, found := strings.Cut(strings.TrimSpace(rest), " (")
		if !found || !strings.HasSuffix(field, ")") {
			continue
		}
		r, ok := index[name]
		if !ok {
			r = &Remote{Name: name}
			index[name] = r
		}
		switch strings.TrimSuffix(field, ")") {
		case "fetch":
			if r.Fetch == "" {
				r.Fetch = url
			}
		case "push":
			if r.Push == "" {
				r.Push = url
			}
		}
	}

	if len(index) == 0 {
		return nil
	}
	remotes := make([]Remote, 0, len(index))
	for _, r := range index {
		remotes = append(remotes, *r)
	}
	// 按名字排序，保证输出稳定
	sort.Slice(remotes, func(i, j int) bool { return remotes[i].Name < remotes[j].Name })
	return remotes
}

// RemoteUrl 返回 path 处仓库 origin remote 的抓取地址——Remotes 的 origin 特化查找，
// 多 URL 取第一条的口径与 Remotes 一致。无 origin remote 时返回 ("", nil)，不视为错误。
func RemoteUrl(path string) (string, error) {
	remotes, err := Remotes(path)
	if err != nil {
		return "", err
	}
	for _, r := range remotes {
		if r.Name == "origin" {
			return r.Fetch, nil
		}
	}
	return "", nil
}
