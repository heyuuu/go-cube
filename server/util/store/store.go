// Package store 收敛文件存储的通用原语：原子写、JSON 整文件读写、JSONL 行式读写。
//
// 只依赖标准库 + slog，路径由调用方给定，无环境副作用（能力层纪律）。
// 只收「已有消费方」的原语，不预埋无消费方的 API。
// 统一契约：所有写操作（WriteFileAtomic / SaveJson / AppendJsonl）自动递归创建父目录。
// 各文件按主题内聚（atomic/json/jsonl），主题之间无依赖。
package store

import "errors"

// ErrFileMissing 是 LoadJson / LoadJsonl 对「文件不存在」的可区分哨兵错误，
// 调用方据此时降级（区别于文件存在但读失败/解析失败——那类错误同样原样上抛，
// 但不应被当作空数据）。
var ErrFileMissing = errors.New("文件不存在")
