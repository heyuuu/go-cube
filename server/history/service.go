package history

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

// retentionDays 日志保留天数。
// 单用户、写入面窄（估算一年 ~7300 行/表），30 天足够保留频率信号且不会无限增长。
const retentionDays = 30

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		db: db,
	}
}

// OnAppCreated 建表（app 构造完成、db 就绪后由 app 层钩子调用）。
func (s *Service) OnAppCreated() error {
	if err := s.db.AutoMigrate(&ProjectSelectLog{}, &ProjectOpenLog{}); err != nil {
		return fmt.Errorf("history 数据表结构迁移失败: %w", err)
	}
	return nil
}

// OnServerStart 清理一次过期日志（仅常驻 server 调用；保留期 30 天，重启频率足够覆盖，无需定时任务）。
// 异步执行，不阻塞 server 启动。
func (s *Service) OnServerStart() {
	go s.purge()
}

// purge 按保留期清理一次过期日志。失败不抛出（降级优先）：只 slog 记录。
func (s *Service) purge() {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	deleted, err := s.PurgeBefore(cutoff)
	if err != nil {
		slog.Warn("清理 history 过期日志失败", "err", err)
		return
	}
	if deleted > 0 {
		slog.Debug("清理 history 过期日志", "deleted", deleted)
	}
}

// PurgeBefore 删除 created_at 早于 cutoff 的日志（两张表），返回删除总行数。
func (s *Service) PurgeBefore(cutoff time.Time) (int64, error) {
	var total int64
	for _, model := range []any{&ProjectSelectLog{}, &ProjectOpenLog{}} {
		res := s.db.Unscoped().Where("created_at < ?", cutoff).Delete(model)
		if res.Error != nil {
			return total, fmt.Errorf("删除过期日志失败: %w", res.Error)
		}
		total += res.RowsAffected
	}
	return total, nil
}

func (s *Service) AddProjectSelectLog(project string, alfred bool) error {
	s.db.Create(&ProjectSelectLog{
		Project: project,
		Alfred:  alfred,
	})
	return nil
}

func (s *Service) LeastSelectedProjects(limit int, alfred bool) []string {
	var projects []string
	s.db.Model(&ProjectSelectLog{}).
		Select("project").
		Where(&ProjectSelectLog{
			Alfred: alfred,
		}).
		Group("project").
		Order("max(id) desc").
		Limit(limit).
		Find(&projects)

	return projects
}

func (s *Service) AddProjectOpenLog(project string, opener string, alfred bool) error {
	s.db.Create(&ProjectOpenLog{
		Project: project,
		Opener:  opener,
		Alfred:  alfred,
	})
	return nil
}

func (s *Service) LeastProjectOpenApps(project string, limit int, alfred bool) []string {
	var projects []string
	s.db.Model(&ProjectOpenLog{}).
		Select("opener").
		Where(&ProjectOpenLog{
			Project: project,
			Alfred:  alfred,
		}).
		Group("opener").
		Order("max(id) desc").
		Limit(limit).
		Find(&projects)

	return projects
}
