package db

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Init(dsn string, models ...any) (*gorm.DB, error) {
	// 连接到 SQLite 数据库
	slog.Info("初始化 db", "dsn", dsn)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: newGormLogger(),
	})
	if err != nil {
		return nil, fmt.Errorf("无法连接到数据库: %w", err)
	}

	// 自动迁移数据表结构
	if len(models) > 0 {
		err = db.AutoMigrate(models...)
		if err != nil {
			return nil, fmt.Errorf("数据表结构迁移失败: %w", err)
		}
	}

	return db, nil
}

func newGormLogger() logger.Interface {
	return logger.NewSlogLogger(slog.Default(), logger.Config{
		LogLevel:                  logger.Info, // 设置最高级别，透传所有日志。具体日志级别处理由 slog 具体 handler 过滤。
		SlowThreshold:             200 * time.Millisecond,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      false,
	})
}
