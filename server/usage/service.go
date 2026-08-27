// Package usage 统一的「项目使用记录」：所有打开入口（web / CLI / alfred）都追加一份
// JSONL 记录，读侧服务于项目列表的最近使用置顶排序与 opener 排序偏好。
//
// 存储：配置目录下 usage.jsonl，追加走 O_APPEND（POSIX 小块 append 原子，CLI 短进程与
// 常驻 server 并发追加安全，全程无锁）；清理靠 server 启动时的一次 compaction。
package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"cube/util/store"
)

// retentionDays 记录保留天数。30 天足够保留频率信号且不会无限增长；
// 每个 (project, opener) 组合的最新一条不受保留期约束（否则排序信号会整体消失）。
const retentionDays = 30

type Service struct {
	path string // usage.jsonl 路径
}

func NewService(path string) *Service {
	return &Service{path: path}
}

// RecordOpen 追加一条打开记录（打开成功后由出口层调用）。
//
// project：主项目绝对路径（归并键）——worktree / monorepo 子目录打开时也恒记主项目路径，
// 排序与 opener 偏好信号不因打开目标不同而分裂。
// opener：opener 名（settings openers 节的 key）。
// dir：实际打开的目标目录绝对路径。打开主项目根时传空（字段省略）；打开 worktree
// 或 workspace 子目录时传其绝对路径（1030/1032 落地后生效）。
func (s *Service) RecordOpen(project string, opener string, dir string) error {
	rec := Record{Time: time.Now(), Project: project, Opener: opener, Dir: dir}
	if err := store.AppendJsonl(s.path, rec); err != nil {
		return fmt.Errorf("写入 usage 记录失败: %w", err)
	}
	return nil
}

// LatestByProject 按 project 去重取最新，返回 path → 最近使用时间。
// 项目列表置顶排序与 lastUsedAt 展示都用它。
func (s *Service) LatestByProject() map[string]time.Time {
	latest := map[string]time.Time{}
	for _, rec := range s.load() {
		// 行序与时间单调一致，后读到的同 key 行更新，直接覆盖
		latest[rec.Project] = rec.Time
	}
	return latest
}

// LatestOpeners 限定 project 按 opener 去重取最新，返回按最近使用倒序的 opener 名。
// 服务于 alfred 的 opener 排序偏好（原 LeastProjectOpenApps）。
func (s *Service) LatestOpeners(project string, limit int) []string {
	latest := map[string]time.Time{}
	for _, rec := range s.load() {
		if rec.Project == project && rec.Opener != "" {
			latest[rec.Opener] = rec.Time
		}
	}
	openers := make([]string, 0, len(latest))
	for name := range latest {
		openers = append(openers, name)
	}
	slices.SortFunc(openers, func(a, b string) int {
		if c := latest[b].Compare(latest[a]); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	if len(openers) > limit {
		openers = openers[:limit]
	}
	return openers
}

// load 全量读取记录。文件缺失视为空（新环境/首次启动），坏行由 store 层跳过。
func (s *Service) load() []Record {
	recs, err := store.LoadJsonl[Record](s.path)
	if err != nil {
		if !errors.Is(err, store.ErrFileMissing) {
			slog.Warn("读取 usage 记录失败", "path", s.path, "err", err)
		}
		return nil
	}
	return recs
}

// OnServerStart compaction 一次（仅常驻 server 调用）。异步执行，不阻塞启动。
func (s *Service) OnServerStart() {
	go func() {
		if removed, err := s.compact(); err != nil {
			slog.Warn("压缩 usage 记录失败", "err", err)
		} else if removed > 0 {
			slog.Debug("压缩 usage 记录", "removed", removed)
		}
	}()
}

// compact 重写文件：每个 (project, opener, dir) 组合留最新一条 + 保留期内记录。
// tmp+rename 整体重写，重写瞬间并发 append 的极少数记录会丢——usage 是 best-effort 信号，接受。
func (s *Service) compact() (int, error) {
	recs := s.load()
	if recs == nil {
		return 0, nil
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	// 组合键含 dir：同项目同 opener 打开不同目标目录（worktree/workspace）各有各的最新一条
	latest := map[string]int{} // "project\x00opener\x00dir" → 最新行下标
	for i, rec := range recs {
		latest[rec.Project+"\x00"+rec.Opener+"\x00"+rec.Dir] = i
	}

	kept := make([]Record, 0, len(recs))
	for i, rec := range recs {
		if i == latest[rec.Project+"\x00"+rec.Opener+"\x00"+rec.Dir] || rec.Time.After(cutoff) {
			kept = append(kept, rec)
		}
	}
	if len(kept) == len(recs) {
		return 0, nil
	}

	var buf []byte
	for _, rec := range kept {
		line, err := json.Marshal(rec)
		if err != nil {
			return 0, fmt.Errorf("序列化 usage 记录失败: %w", err)
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	if err := store.WriteFileAtomic(s.path, buf, 0644); err != nil {
		return 0, fmt.Errorf("重写 usage 记录失败: %w", err)
	}
	return len(recs) - len(kept), nil
}
