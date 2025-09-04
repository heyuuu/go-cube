package logger

import (
	"bytes"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"time"
)

const (
	segString   = "string"
	segTime     = "time"
	segLevel    = "level"
	segMessage  = "message"
	segFunction = "function"
	segFile     = "file"
	segLine     = "line"
	segAttrs    = "attrs"
)

type formatSegment struct {
	typ string
	arg string
}
type formatRule []formatSegment

func parseFormatRule(s string) formatRule {
	var rule formatRule

	usedIndex := 0   // 已处理的位置
	braceIndex := -1 // 当前大括号开始位置
	colonIndex := -1 // 当前大括号后第一个冒号位置
	for i, c := range []byte(s) {
		switch c {
		case '{':
			braceIndex = i
			colonIndex = -1
		case ':':
			if colonIndex < 0 {
				colonIndex = i
			}
		case '}':
			var typ, arg string
			if colonIndex > 0 {
				typ, arg = s[braceIndex+1:colonIndex], s[colonIndex+1:i]
			} else {
				typ, arg = s[braceIndex+1:i], ""
			}
			switch typ {
			case segTime, segLevel, segMessage, segFunction, segFile, segLine, segAttrs:
				if usedIndex < braceIndex {
					rule = append(rule, formatSegment{segString, s[usedIndex:braceIndex]})
				}
				rule = append(rule, formatSegment{typ: typ, arg: arg})
				usedIndex = i + 1
			}
		}
	}
	if usedIndex < braceIndex {
		rule = append(rule, formatSegment{segString, s[usedIndex:]})
	}

	return rule
}

func formatRecord(rule formatRule, r slog.Record) []byte {
	// source
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()

	var buf bytes.Buffer
	for _, seg := range rule {
		switch seg.typ {
		case segString:
			buf.WriteString(seg.arg)
		case segTime:
			layout := seg.arg
			if layout == "" {
				layout = time.RFC3339
			}
			buf.WriteString(r.Time.Format(layout))
		case segLevel:
			buf.WriteString(r.Level.String())
		case segMessage:
			buf.WriteString(r.Message)
		case segFunction:
			buf.WriteString(f.Function)
		case segFile:
			buf.WriteString(f.File)
		case segLine:
			buf.WriteString(strconv.Itoa(f.Line))
		case segAttrs:
			r.Attrs(func(a slog.Attr) bool {
				buf.WriteString(" ")
				buf.WriteString(a.Key)
				buf.WriteString("=")
				buf.WriteString(fmt.Sprintf("%v", a.Value.Any()))
				return true
			})
		}
	}
	buf.WriteByte('\n')
	return buf.Bytes()
}
