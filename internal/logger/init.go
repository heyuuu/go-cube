package logger

import (
	"github.com/heyuuu/go-cube/internal/config"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

var initialize bool

func Init() {
	// check initialize
	if initialize {
		slog.Warn("logger already initialized")
		return
	}
	initialize = true

	// init
	logger := slog.New(initHandler())
	slog.SetDefault(logger)

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
func initHandler() slog.Handler {
	var fileHandler, stdioHandler slog.Handler

	// 初始化日志文件
	fileHandler = initFileHandler()

	// 在 Debug 模式下或日志文件不生效时，初始化标准 io handler
	if config.IsDebug() || fileHandler == nil {
		stdioHandler = initStdioHandler()
	}

	// 返回
	return newMultiHandler(fileHandler, stdioHandler)
}

// 初始化日志文件 handler
func initFileHandler() slog.Handler {
	conf := config.Default()

	// path
	path := conf.LogPath
	if path == "" {
		path = config.ConfigPath()
	}

	// init log file
	filePath := filepath.Join(path, logFileName)
	file, err := os.Create(filePath)
	if err != nil {
		lazyLog(func() {
			slog.Error("open log file failed", "logFile", filePath, "err", err)
		})
		return nil
	}
	lazyLog(func() {
		slog.Debug("init logger file succeed", "logFile", filePath)
	})

	// level
	level := parseLogLevel(conf.LogLevel)

	// format
	format := conf.LogFormat
	if format == "" {
		format = logFileFormat
	}

	// init handler
	return newSingleHandler(level, format, file)
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
