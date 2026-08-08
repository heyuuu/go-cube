package logger

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"cube/config"
)

const logFileName = "cube.log"
const logFileFormat = "[{time:2006-01-02 15:04:05.000} {level} {file}:{line}] {message}.{attrs}"
const logStdioFormat = "[{time:15:04:05.000} {level} {file}:{line}] {message}.{attrs}"

var logStdioColors = map[slog.Level]string{
	slog.LevelDebug: colorGreen,
	slog.LevelInfo:  colorCyan,
	slog.LevelWarn:  colorYellow,
	slog.LevelError: colorRed,
}

func Init(cfg config.LogConfig) {
	handler := initHandler(cfg)
	slog.SetDefault(slog.New(handler))

	// 延迟日志
	applyLazyLogs()
}

// 延迟日志
var lazyLogs []func()

func lazyLog(f func()) {
	if f != nil {
		lazyLogs = append(lazyLogs, f)
	}
}
func applyLazyLogs() {
	for len(lazyLogs) > 0 {
		f := lazyLogs[0]
		lazyLogs = lazyLogs[1:]
		f()
	}
}
func initHandler(cfg config.LogConfig) slog.Handler {
	var fileHandler, stdioHandler slog.Handler

	// 初始化日志文件
	fileHandler = initFileHandler(cfg)

	// 在 Debug 模式下或日志文件不生效时，初始化标准 io handler
	if config.IsDebug() || fileHandler == nil {
		stdioHandler = initStdioHandler()
	}

	// 返回
	return newMultiHandler(fileHandler, stdioHandler)
}

// 初始化日志文件 handler
func initFileHandler(cfg config.LogConfig) slog.Handler {
	// path
	logPath := cfg.Path
	if logPath == "" {
		fmt.Printf("log path 配置为空，不记录日志文件")
		return nil
	}
	logFile := filepath.Join(logPath, logFileName)

	// init log file
	file, err := os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		lazyLog(func() {
			slog.Error("open log file failed", "logFile", logFile, "err", err)
		})
		return nil
	}

	// level
	level := parseLogLevel(cfg.Level)

	// format
	format := cfg.Format
	if format == "" {
		format = logFileFormat
	}

	// init handler
	h := newSingleHandler(level, format, file)

	// 记录 handler 信息
	lazyLog(func() {
		slog.Debug("init log file handler succeed", "level", level.String(), "logFile", logFile)
	})

	return h
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

func initStdioHandler() slog.Handler {
	level := slog.LevelDebug
	format := logStdioFormat

	h := newSingleHandler(level, format, os.Stderr)
	h.UseAnsiColor(logStdioColors)
	return h
}
