package project

import (
	"testing"
	"time"

	"cube/internal/testfixture"
)

// TestStartRefreshTicker_FreshCacheSkippedStartupRefresh 磁盘缓存仍新鲜时，
// StartRefreshTicker 启动即刷的一次应被跳过（air 热重载场景不重采）：
// 以 scanUpdatedAt 是否保持零值（未被 refresh 触碰）作为「未刷新」的判据。
func TestStartRefreshTicker_FreshCacheSkippedStartupRefresh(t *testing.T) {
	s := newServiceAt(t, testfixture.NewWorkspace(t).Dir, "g1", 1)
	if err := s.gitCache.Save(); err != nil { // 预写一份「刚落盘」的新鲜缓存
		t.Fatalf("预写缓存失败: %v", err)
	}
	s.StartRefreshTicker(time.Hour)
	defer s.StopRefreshTicker()

	if !s.ScanUpdatedAt().IsZero() {
		t.Fatalf("缓存新鲜时应跳过启动刷新，scanUpdatedAt 却被更新: %v", s.ScanUpdatedAt())
	}
}

// TestStartRefreshTicker_StaleCacheTriggersStartupRefresh 缓存过期（或为空）时，
// 启动仍应立即刷新一次，避免冷启动空窗。
func TestStartRefreshTicker_StaleCacheTriggersStartupRefresh(t *testing.T) {
	s := newServiceAt(t, testfixture.NewWorkspace(t).Dir, "g1", 1)
	s.StartRefreshTicker(time.Hour)
	defer s.StopRefreshTicker()

	deadline := time.Now().Add(2 * time.Second)
	for s.ScanUpdatedAt().IsZero() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if s.ScanUpdatedAt().IsZero() {
		t.Fatal("缓存过期时应启动即刷一次，scanUpdatedAt 保持零值")
	}
}
