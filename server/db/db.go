package db

import (
	"fmt"
	"log/slog"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"cube/config"
)

func Init(dsn string, models ...any) (*gorm.DB, error) {
	// 连接到 SQLite 数据库
	slog.Info("init db", "dsn", dsn)

	gormConfig := &gorm.Config{}
	if config.IsDebug() {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	}
	db, err := gorm.Open(sqlite.Open(dsn), gormConfig)
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
