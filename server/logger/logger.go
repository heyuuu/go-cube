package logger

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lmittmann/tint"

	"cube/config"
	"cube/util/tui"
)

const logFileName = "log/cube.log"
const logTimeFormat = "2006-01-02 15:04:05.000"
const stdioLogTimeFormat = "15:04:05.000"

// Init 初始化日志
// 初始化失败会直接 panic，因为没有日志根本无法记录错误，容易导致静默失败。
func Init(cfg config.LogConfig, debug bool) {
	level := parseLogLevel(cfg.Level)

	// 文件日志 Handler，始终启用
	handler := initFileHandler(level, cfg.Path)

	// 在 Debug 模式下时，启用标准输出日志 Handler，颜色看 stderr 是否 TTY
	if debug {
		stdioHandler := initStdioHandler(level)
		handler = slog.NewMultiHandler(handler, stdioHandler)
	}

	// 设置为 slog 默认 handler
	slog.SetDefault(slog.New(handler))
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func initFileHandler(level slog.Level, logPath string) slog.Handler {
	if logPath == "" {
		panic("log path 配置不应为空，请检查 LogConfig 配置")
	}
	logFile := filepath.Join(logPath, logFileName)

	// init log file
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		panic(fmt.Errorf("创建日志目录失败: dir=%s err=%w", filepath.Dir(logPath), err))
	}
	file, err := os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		panic(fmt.Errorf("无法开始日志文件: file=%s err=%w", logFile, err))
	}

	return slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: level,
		// 修改日志格式
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// 时间字段：顶层记录的 time（groups 为空，key 为 "time"）
			if len(groups) == 0 && a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format(logTimeFormat))
				}
			}
			return a
		},
	})
}

func initStdioHandler(level slog.Level) slog.Handler {
	return tint.NewTextHandler(os.Stderr, &tint.Options{
		Level:     level,
		AddSource: true,
		NoColor:   !tui.IsTTY(), // 仅 TTY 模式使用 ANSI 颜色
		// 修改日志格式
		TimeFormat: stdioLogTimeFormat,
	})
}
