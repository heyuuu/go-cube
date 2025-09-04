package logger

import (
	"log/slog"
	"os"
)

// const lineFormat = "[{level} {time:2006-01-02 15:04:05.000} {function}:{file}:{line}] {message} {attrs}"
const lineFormat = "[{level} {time:2006-01-02 15:04:05.000} {file}:{line}] {message} {attrs}"

func Init(path string) {
	h := &textFormatHandler{
		rule:     parseFormatRule(lineFormat),
		minLevel: slog.LevelInfo,
		w:        os.Stdout,
	}
	l := slog.New(h)
	slog.SetDefault(l)
}
