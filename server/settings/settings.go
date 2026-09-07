// Package settings 管理配置目录下 settings.json 的节级读写。
// settings.json 是「运行中可变更的用户可管理数据」的载体（与 config.json 切割：
// 后者启动加载、不热更），形状为多节 JSON 文档，各领域只读写自己的节。
//
// 职责边界：本包只管机制（节透传、原子写、读降级），不感知任何节名与节内结构；
// 节名常量与节内校验归各领域包（如 opener 包的 "openers" 节）。
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"cube/util/store"
)

// doc 文档内存形态：节名 → 原始 JSON。非目标节语义级透传——值原样保留不解析，
// 但 Unmarshal 落 RawMessage 时会压缩空白，存盘经统一缩进，其他节的格式会被规范化。
type doc map[string]json.RawMessage

// fileMu 串行化单进程内的读-改-写（web server 多请求 goroutine 并发真实存在）。
// 仅保证单次 SaveSection 原子；调用方跨两次调用的组合竞态不在保障范围（单写者假设）。
var fileMu sync.Mutex

// LoadSection 从 file 读出 section 节并反序列化到 dst（传指针）。
// 一切读失败形态（文件不存在 / 坏 JSON / 节缺失 / 节格式坏）统一降级：
// 记日志、dst 保持零值、不返回 error——读路径永不阻塞调用方。
func LoadSection(file, section string, dst any) {
	d, err := readDoc(file)
	if err != nil {
		slog.Warn("读取 settings.json 失败，按空配置继续", "file", file, "err", err)
		return
	}
	raw, ok := d[section]
	if !ok {
		return
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		slog.Warn("settings.json 节格式错误，该节按空配置继续", "file", file, "section", section, "err", err)
	}
}

// SaveSection 锁内「读全文档 → 仅替换 section 节 → 原子写回」。
// 文件为坏 JSON 时拒绝写入，防止静默覆盖掉手工搞坏的其他节；
// 注意：src 为 nil 会把节写成 "null"，调用方别传 nil（无「删节」语义）。
func SaveSection(file, section string, src any) error {
	fileMu.Lock()
	defer fileMu.Unlock()

	raw, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("序列化节失败: section=%s err=%w", section, err)
	}
	d, err := readDoc(file)
	if err != nil {
		return fmt.Errorf("拒绝写入，settings.json 需手工修复: file=%s err=%w", file, err)
	}
	d[section] = raw
	return saveDoc(file, d)
}

// readDoc 读入并解析文档（store.LoadJson）。文件不存在是正常态（首次运行）→ 空 doc；
// 坏 JSON 是异常态 → 返回错误（读侧由调用方降级，写侧用于拒写）。
func readDoc(file string) (doc, error) {
	d, err := store.LoadJson[doc](file)
	if errors.Is(err, store.ErrFileMissing) {
		return doc{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("settings.json 读取/解析失败: file=%s err=%w", file, err)
	}
	return d, nil
}

// saveDoc 原子写入（store.SaveJson：缩进 + tmp/rename）。
// 顶层键序由 json.Marshal 的 map 字典序决定——稳定且 diff 友好，不引入 OrderedMap。
func saveDoc(file string, d doc) error {
	if err := store.SaveJson(file, d); err != nil {
		return fmt.Errorf("写入 settings.json 失败: file=%s err=%w", file, err)
	}
	return nil
}
