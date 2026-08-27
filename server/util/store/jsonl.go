package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
)

// AppendJsonl 序列化 v 为单行 JSON 追加写入 path（不存在则创建，父目录自动递归创建）。
// O_APPEND 小块写由 POSIX 保证原子，无需跨进程锁（单写者追加语义）。
func AppendJsonl[T any](path string, v T) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("序列化 JSONL 行失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开 JSONL 文件失败: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(bytes.TrimSpace(data), '\n')); err != nil {
		return fmt.Errorf("追加 JSONL 行失败: %w", err)
	}
	return nil
}

// WriteJsonl 序列化 items 为逐行 JSON，整体原子重写 path（tmp+rename，父目录自动创建）。
// 用于 compaction 类「全量重写」场景；常规追加走 AppendJsonl——重写瞬间并发 append 的记录会丢，
// 调用方须自行接受该语义。
func WriteJsonl[T any](path string, items []T) error {
	var buf []byte
	for _, item := range items {
		line, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("序列化 JSONL 行失败: %w", err)
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	if err := WriteFileAtomic(path, buf, 0644); err != nil {
		return fmt.Errorf("重写 JSONL 文件失败: %w", err)
	}
	return nil
}

// LoadJsonl 全量读取 path 的 JSONL 内容，按行反序列化。
// 文件缺失返回 (nil, ErrFileMissing)；坏行跳过并 slog 记录（降级优先，不因个别脏数据丢弃整份文件）。
func LoadJsonl[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrFileMissing
		}
		return nil, fmt.Errorf("打开 JSONL 文件失败: %w", err)
	}
	defer f.Close()

	var items []T
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // 单行上限 4MB，防超长行撑爆默认 64KB 缓冲
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var v T
		if err := json.Unmarshal(line, &v); err != nil {
			slog.Warn("JSONL 坏行已跳过", "path", path, "line", lineNo, "err", err)
			continue
		}
		items = append(items, v)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取 JSONL 文件失败: %w", err)
	}
	return items, nil
}

// IterJsonl 正向逐行流式读取 path 的 JSONL 内容，yield (物理行号, 记录)。
// 逐行推进不整读文件，适合大文件；调用方随时 break 即停止读取（iter 天然支持）。
// 降级语义同 LoadJsonl，但迭代器无法返回 error——文件缺失静默结束、打开失败只 slog 记录；
// 空行跳过、坏行跳过并 slog 记录，行号始终是文件中的物理行号。
func IterJsonl[T any](path string) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		f, err := os.Open(path)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				slog.Warn("打开 JSONL 文件失败", "path", path, "err", err)
			}
			return
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			var v T
			if err := json.Unmarshal(line, &v); err != nil {
				slog.Warn("JSONL 坏行已跳过", "path", path, "line", lineNo, "err", err)
				continue
			}
			if !yield(lineNo, v) {
				return
			}
		}
	}
}

// reverseChunk 逆向逐行读的分块大小；行数据通常远小于此，一次分块即可覆盖多行。
const reverseChunk = 64 * 1024

// IterJsonlReverse 逆向（从文件尾到文件头）逐行流式读取 path 的 JSONL 内容，
// yield (物理行号, 记录)。分块从尾部往前扫描，不整读文件，适合大文件取最近记录的场景。
// 降级语义与行号口径同 IterJsonl。
func IterJsonlReverse[T any](path string) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		f, err := os.Open(path)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				slog.Warn("打开 JSONL 文件失败", "path", path, "err", err)
			}
			return
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil || info.Size() == 0 {
			return
		}
		newlines, endsNL := scanNewlines(f, info.Size())
		totalLines := newlines
		if !endsNL {
			totalLines++ // 最后一行没有 '\n' 终止，也计入
		}

		emit := func(line []byte, lineNo int) bool {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				return true // 空行占行号但不出记录
			}
			var v T
			if err := json.Unmarshal(line, &v); err != nil {
				slog.Warn("JSONL 坏行已跳过", "path", path, "line", lineNo, "err", err)
				return true
			}
			return yield(lineNo, v)
		}

		lineNo := totalLines
		// 文件以 '\n' 结尾时，最后一个 '\n' 之后不存在内容行，首次提取的空段不消耗行号
		skipTail := endsNL
		pending := []byte{} // 尚未凑齐的行首部分（块向前推进时不断向后拼接）
		offset := info.Size()
		buf := make([]byte, reverseChunk)
		for offset > 0 {
			n := min(reverseChunk, int(offset))
			offset -= int64(n)
			if _, err := f.ReadAt(buf[:n], offset); err != nil {
				slog.Warn("逆向读取 JSONL 文件失败", "path", path, "err", err)
				return
			}
			// cur = 本块 + 之前的不完整尾巴（顺序：块在前、尾巴在后）
			cur := make([]byte, 0, n+len(pending))
			cur = append(cur, buf[:n]...)
			cur = append(cur, pending...)
			for {
				idx := bytes.LastIndexByte(cur, '\n')
				if idx < 0 {
					break
				}
				line := cur[idx+1:]
				cur = cur[:idx]
				if skipTail {
					skipTail = false
					continue
				}
				if !emit(line, lineNo) {
					return
				}
				lineNo--
			}
			pending = cur
		}
		// 循环结束后 pending 是文件第一行（整个循环没提取过任何行时它就是唯一一行）
		emit(pending, lineNo)
	}
}

// scanNewlines 统计文件的 '\n' 数量，并返回文件是否以 '\n' 结尾。
func scanNewlines(f *os.File, size int64) (newlines int, endsNL bool) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, false
	}
	defer f.Seek(0, io.SeekStart)
	buf := make([]byte, 64*1024)
	for {
		n, err := f.Read(buf)
		newlines += bytes.Count(buf[:n], []byte{'\n'})
		if n == 0 || err != nil {
			break
		}
	}
	var last [1]byte
	if _, err := f.ReadAt(last[:], size-1); err == nil {
		endsNL = last[0] == '\n'
	}
	return newlines, endsNL
}
