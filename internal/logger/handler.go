package logger

import (
	"context"
	"errors"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
	"io"
	"log/slog"
)

// ansi colors
const (
	colorBlack   = "\033[30m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m" // 品红
	colorCyan    = "\033[36m" // 青色
	colorWhite   = "\033[37m"
	colorReset   = "\033[0m"
)

// singleHandler
type singleHandler struct {
	rule         formatRule
	minLevel     slog.Level
	w            io.Writer
	useAnsiColor bool
	ansiColor    map[slog.Level]string
}

func newSingleHandler(minLevel slog.Level, format string, w io.Writer) *singleHandler {
	return &singleHandler{
		rule:     parseFormatRule(format),
		minLevel: minLevel,
		w:        w,
	}
}

func (h *singleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.minLevel
}

func (h *singleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// todo
	return h
}

func (h *singleHandler) WithGroup(name string) slog.Handler {
	// todo
	return h
}

func (h *singleHandler) Handle(ctx context.Context, record slog.Record) error {
	var bytes []byte
	if !h.useAnsiColor {
		bytes = formatRecord(h.rule, record)
	} else {
		color := h.ansiColor[record.Level]
		reset := colorReset
		bytes = formatRecordEx(h.rule, record, color, reset)
	}

	_, err := h.w.Write(bytes)
	return err
}

func (h *singleHandler) UseAnsiColor(ansiColor map[slog.Level]string) {
	if ansiColor == nil {
		h.useAnsiColor = false
		h.ansiColor = nil
	} else {
		h.useAnsiColor = true
		h.ansiColor = ansiColor
	}
}

type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(rawHandlers ...slog.Handler) slog.Handler {
	var handlers []slog.Handler
	for _, h := range rawHandlers {
		if h == nil {
			continue
		}
		if mh, ok := h.(*multiHandler); ok {
			handlers = append(handlers, mh.handlers...)
		} else {
			handlers = append(handlers, h)
		}
	}
	if len(handlers) == 1 {
		return handlers[0]
	}
	return &multiHandler{
		handlers: handlers,
	}
}

func (mh *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range mh.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (mh *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(mh.handlers) == 0 {
		return mh
	}

	handlers := slicekit.Map(mh.handlers, func(h slog.Handler) slog.Handler {
		return h.WithAttrs(attrs)
	})
	return newMultiHandler(handlers...)
}

func (mh *multiHandler) WithGroup(name string) slog.Handler {
	if len(mh.handlers) == 0 {
		return mh
	}

	handlers := slicekit.Map(mh.handlers, func(h slog.Handler) slog.Handler {
		return h.WithGroup(name)
	})
	return newMultiHandler(handlers...)
}

func (mh *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var errs []error
	for _, h := range mh.handlers {
		err := h.Handle(ctx, record)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
