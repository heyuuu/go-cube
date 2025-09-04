package logger

import (
	"context"
	"io"
	"log/slog"
)

type textFormatHandler struct {
	rule     formatRule
	minLevel slog.Level
	w        io.Writer
}

func (h *textFormatHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.minLevel
}

func (h *textFormatHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *textFormatHandler) WithGroup(name string) slog.Handler {
	return h
}

func (h *textFormatHandler) Handle(ctx context.Context, record slog.Record) error {
	bytes := formatRecord(h.rule, record)
	_, err := h.w.Write(bytes)
	return err
}
