package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// SaveJson 序列化 v 并原子写入 path。
// 用缩进 + 尾换行——JSON 存储文件常被人工翻看，可读性优先于体积。
func SaveJson(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 JSON 失败: %w", err)
	}
	return WriteFileAtomic(path, append(data, '\n'), 0644)
}

// LoadJson 读取并反序列化 path 的 JSON 内容。
// 文件缺失返回 (零值, ErrFileMissing)；解析失败原样上抛，由调用方决定降级策略。
func LoadJson[T any](path string) (T, error) {
	var zero T
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return zero, ErrFileMissing
		}
		return zero, fmt.Errorf("读取 JSON 文件失败: %w", err)
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return zero, fmt.Errorf("解析 JSON 文件失败: %w", err)
	}
	return v, nil
}
