package usage

import (
	"testing"
	"time"

	"cube/internal/testfixture"
	"cube/util/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	ws := testfixture.NewWorkspace(t)
	return NewService(ws.Join("usage.jsonl"))
}

func TestRecordOpen_AndLatestByProject(t *testing.T) {
	s := newTestService(t)

	if err := s.RecordOpen("/p/a", "idea", ""); err != nil {
		t.Fatalf("RecordOpen 失败: %v", err)
	}
	s.RecordOpen("/p/b", "code", "")
	s.RecordOpen("/p/a", "code", "") // a 用 2 次，最新 opener 是 code

	latest := s.LatestByProject()
	if len(latest) != 2 {
		t.Fatalf("按 project 去重应剩 2 条，实际 %v", latest)
	}
	timeA, ok := latest["/p/a"]
	if !ok || timeA.IsZero() {
		t.Fatalf("缺少 /p/a 的最新时间: %v", latest)
	}
	if timeA.Before(latest["/p/b"]) {
		t.Fatalf("/p/a 最新使用应不早于 /p/b")
	}
}

func TestLatestByProject_Empty(t *testing.T) {
	s := newTestService(t)
	if got := s.LatestByProject(); len(got) != 0 {
		t.Fatalf("无记录应返回空 map，实际 %v", got)
	}
}

func TestLatestOpeners(t *testing.T) {
	s := newTestService(t)

	s.RecordOpen("/p/proj", "code", "")
	s.RecordOpen("/p/proj", "idea", "")
	s.RecordOpen("/p/proj", "code", "") // code 最后一次最新
	s.RecordOpen("/p/other", "vim", "") // 无关项目

	got := s.LatestOpeners("/p/proj", 10)
	if len(got) != 2 || got[0] != "code" || got[1] != "idea" {
		t.Fatalf("LatestOpeners 顺序异常: %v", got)
	}

	// limit 截断
	if got := s.LatestOpeners("/p/proj", 1); len(got) != 1 || got[0] != "code" {
		t.Fatalf("limit=1 应只留 code，实际 %v", got)
	}

	// 不存在的 project
	if got := s.LatestOpeners("/p/nonexistent", 10); len(got) != 0 {
		t.Fatalf("不存在的 project 应返回空，实际 %v", got)
	}
}

func TestCompact(t *testing.T) {
	s := newTestService(t)

	now := time.Now()
	old := now.AddDate(0, 0, -40)
	// /p/old：两条超出保留期，compact 后只留组合最新一条（排序信号不随保留期消失）
	writeRecord(t, s.usageFilePath, Record{Time: old.Add(-time.Hour), Project: "/p/old", Opener: "code"})
	writeRecord(t, s.usageFilePath, Record{Time: old, Project: "/p/old", Opener: "code"})
	// /p/recent：组合旧记录超出保留期（被删），最新一条在保留期内（留）
	writeRecord(t, s.usageFilePath, Record{Time: old.Add(time.Second), Project: "/p/recent", Opener: "vim"})
	writeRecord(t, s.usageFilePath, Record{Time: now.Add(-time.Hour), Project: "/p/recent", Opener: "vim"})
	// /p/mid：保留期内的中间记录不删（保留期内全留）
	writeRecord(t, s.usageFilePath, Record{Time: now.Add(-2 * time.Hour), Project: "/p/mid", Opener: "vim"})
	writeRecord(t, s.usageFilePath, Record{Time: now.Add(-3 * time.Hour), Project: "/p/mid", Opener: "vim"})

	removed, err := s.compact()
	if err != nil {
		t.Fatalf("compact 失败: %v", err)
	}
	if removed != 2 {
		t.Fatalf("应删除 2 条（old 组合旧记录 + recent 组合旧记录），实际 %d", removed)
	}
	recs, _ := store.LoadJsonl[Record](s.usageFilePath)
	if len(recs) != 4 {
		t.Fatalf("compaction 后应剩 4 条，实际 %v", recs)
	}
	if recs[0].Project != "/p/old" || !recs[0].Time.Equal(old) {
		t.Fatalf("old 组合应留最新一条，实际 %v", recs)
	}
	if recs[1].Project != "/p/recent" || !recs[1].Time.After(now.Add(-90*time.Minute)) {
		t.Fatalf("recent 组合应留最新一条，实际 %v", recs)
	}
}

func TestCompact_NothingToDo(t *testing.T) {
	s := newTestService(t)
	if removed, err := s.compact(); err != nil || removed != 0 {
		t.Fatalf("空文件 compact 应无操作，实际 removed=%d err=%v", removed, err)
	}

	s.RecordOpen("/p/a", "code", "")
	if removed, err := s.compact(); err != nil || removed != 0 {
		t.Fatalf("单条记录 compact 应无删除，实际 removed=%d err=%v", removed, err)
	}
}

func writeRecord(t *testing.T, path string, rec Record) {
	t.Helper()
	if err := store.AppendJsonl(path, rec); err != nil {
		t.Fatalf("写入测试记录失败: %v", err)
	}
}

// dir 字段：主项目根打开省略；worktree/workspace 目标记绝对路径；compaction 组合键含 dir
func TestRecordOpen_Dir(t *testing.T) {
	s := newTestService(t)

	// 超期旧记录先写（追加不变量：行序 = 时间序，旧记录必须在文件前部）
	old := time.Now().AddDate(0, 0, -40)
	writeRecord(t, s.usageFilePath, Record{Time: old, Project: "/p/cube", Opener: "idea", Dir: "/p/cube--web/server"})
	writeRecord(t, s.usageFilePath, Record{Time: old, Project: "/p/cube", Opener: "idea", Dir: "/p/cube--web"})
	writeRecord(t, s.usageFilePath, Record{Time: old, Project: "/p/cube", Opener: "idea"}) // 主根

	s.RecordOpen("/p/cube", "idea", "")                    // 主根
	s.RecordOpen("/p/cube", "idea", "/p/cube--web/server") // worktree 内子目录
	s.RecordOpen("/p/cube", "idea", "/p/cube--web/server") // 同目标第二次
	s.RecordOpen("/p/cube", "code", "/p/cube--web")        // worktree 根

	latest := s.LatestByProject()
	if len(latest) != 1 {
		t.Fatalf("dir 不同不应分裂 project 归并键，实际 %v", latest)
	}

	// compaction：组合键含 dir——被更新的超期旧记录删除；
	// idea+/p/cube--web 的旧记录是其组合唯一一条（组合最新），即使超期也保留
	if removed, err := s.compact(); err != nil || removed != 2 {
		t.Fatalf("compact 应删除 2 条超期旧记录，实际 removed=%d err=%v", removed, err)
	}
	recs, _ := store.LoadJsonl[Record](s.usageFilePath)
	if len(recs) != 5 {
		t.Fatalf("compaction 后应剩 5 条（4 条新记录 + 超期组合最新 1 条），实际 %v", recs)
	}
	if recs[0].Opener != "idea" || recs[0].Dir != "/p/cube--web" {
		t.Fatalf("超期组合最新一条应保留，实际 %v", recs[0])
	}
	for _, r := range recs[1:] {
		if r.Time.Before(time.Now().Add(-time.Hour)) {
			t.Fatalf("被更新的超期记录不应保留: %v", r)
		}
	}
}

func TestRecordOpen_DirEqualsProjectNormalizedToEmpty(t *testing.T) {
	s := newTestService(t)

	// dir 等于项目根时应归一为空（「根目录记空」是存储契约），worktree 路径原样保留
	if err := s.RecordOpen("/p/proj", "code", "/p/proj"); err != nil {
		t.Fatalf("RecordOpen 失败: %v", err)
	}
	s.RecordOpen("/p/proj", "code", "/p/proj/.worktrees/wt")

	recs, err := store.LoadJsonl[Record](s.usageFilePath)
	if err != nil {
		t.Fatalf("读取记录失败: %v", err)
	}
	if len(recs) != 2 || recs[0].Dir != "" || recs[1].Dir != "/p/proj/.worktrees/wt" {
		t.Fatalf("dir 归一异常: %+v", recs)
	}
}
