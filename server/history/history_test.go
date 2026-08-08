package history

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestDB 构造一个内存 sqlite 并 AutoMigrate history 的两张表。
// :memory: 零磁盘、毫秒级、每次测试干净。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存 sqlite 失败: %v", err)
	}
	if err := db.AutoMigrate(&ProjectSelectLog{}, &ProjectOpenLog{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	return db
}

func TestAddProjectSelectLog_AndLeast(t *testing.T) {
	s := NewService(newTestDB(t))

	if err := s.AddProjectSelectLog("proj-a", false); err != nil {
		t.Fatalf("Add 失败: %v", err)
	}
	s.AddProjectSelectLog("proj-b", false)
	s.AddProjectSelectLog("proj-c", true) // alfred=true

	// 已知行为（待修）：gorm 的 Where(&struct{Alfred:false}) 把 false 当零值忽略，
	// 导致 alfred=true 的记录也会被查出来。所以传 alfred=false 实际返回全部 3 条。
	// 此测试断言现状；若未来修复了 alfred=false 的过滤，需同步调整。
	got := s.LeastSelectedProjects(10, false)
	if len(got) != 3 {
		t.Fatalf("（现状）alfred=false 未过滤，应返回全部 3 条，实际 %v", got)
	}

	// alfred=true：gorm 对非零值过滤生效，只返回 c
	got = s.LeastSelectedProjects(10, true)
	if len(got) != 1 || got[0] != "proj-c" {
		t.Fatalf("alfred LeastSelectedProjects = %v，期望 [proj-c]", got)
	}
}

func TestLeastSelectedProjects_LimitAndEmpty(t *testing.T) {
	s := NewService(newTestDB(t))

	// 空表
	if got := s.LeastSelectedProjects(5, false); len(got) != 0 {
		t.Fatalf("空表应返回空切片，实际 %v", got)
	}

	s.AddProjectSelectLog("p1", false)
	s.AddProjectSelectLog("p2", false)
	s.AddProjectSelectLog("p3", false)

	// limit 截断
	got := s.LeastSelectedProjects(2, false)
	if len(got) != 2 {
		t.Fatalf("limit=2 应返回 2 个，实际 %v", got)
	}
	// 按 id desc，应是 p3, p2
	if got[0] != "p3" || got[1] != "p2" {
		t.Fatalf("排序异常: %v", got)
	}
}

func TestAddProjectOpenLog_AndLeastApps(t *testing.T) {
	s := NewService(newTestDB(t))

	s.AddProjectOpenLog("proj", "code", false)
	s.AddProjectOpenLog("proj", "idea", false)
	s.AddProjectOpenLog("proj", "code", false) // code 用 2 次
	s.AddProjectOpenLog("other", "vim", false)

	// proj 的 opener 列表（按 max(id) desc）
	got := s.LeastProjectOpenApps("proj", 10, false)
	// code 最后一次 id 最大 → 排前；idea 次之
	if len(got) != 2 {
		t.Fatalf("proj 的 app 数 = %d，期望 2：%v", len(got), got)
	}
	if got[0] != "code" || got[1] != "idea" {
		t.Fatalf("LeastProjectOpenApps 顺序异常: %v", got)
	}

	// 不存在的 project
	got = s.LeastProjectOpenApps("nonexistent", 10, false)
	if len(got) != 0 {
		t.Fatalf("不存在的 project 应返回空，实际 %v", got)
	}
}

func TestLeastProjectOpenApps_AlfredFilter(t *testing.T) {
	s := NewService(newTestDB(t))

	s.AddProjectOpenLog("proj", "code", false)
	s.AddProjectOpenLog("proj", "idea", true) // 仅 alfred

	// 已知行为（待修）：同 TestAddProjectSelectLog_AndLeast，alfred=false 过滤不生效，
	// 实际返回 [idea, code]（按 max(id) desc，idea 的 id 大）。
	got := s.LeastProjectOpenApps("proj", 10, false)
	if len(got) != 2 {
		t.Fatalf("（现状）alfred=false 未过滤，应返回 2 条，实际 %v", got)
	}

	// alfred=true 过滤生效，只见 idea
	got = s.LeastProjectOpenApps("proj", 10, true)
	if len(got) != 1 || got[0] != "idea" {
		t.Fatalf("alfred 过滤异常: %v", got)
	}
}
