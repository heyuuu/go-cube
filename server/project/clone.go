package project

import (
	"path"
	"strings"

	"cube/util/git"
)

// CloneRule 克隆规则
type CloneRule struct {
	RepoHost   string `json:"repoHost"`   // 源域名，无协议. 如: github.com
	RepoPrefix string `json:"repoPrefix"` // uri 前缀，默认为空. 如: "", "/heyuuu"
	LocalPath  string `json:"localPath"`  // 对应本地目录
}

func MatchCloneRule(repoUrl string, rules []CloneRule) (rule CloneRule, localPath string, ok bool) {
	u, err := git.ParseRepoUrl(repoUrl)
	if err != nil {
		return
	}

	for _, r := range rules {
		// 匹配规则
		if u.Host == r.RepoHost && strings.HasPrefix(u.Path, r.RepoPrefix) {
			// 仅第一次匹配，或新匹配规则的 Prefix 长度大于旧规则 Prefix 时，记录新规则
			if !ok || (len(r.RepoPrefix) > len(rule.RepoPrefix)) {
				ok = true
				rule = r
				localPath = path.Join(rule.LocalPath, strings.TrimSuffix(strings.TrimPrefix(u.Path, r.RepoPrefix), ".git"))
			}
		}
	}
	return
}
