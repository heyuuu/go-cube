package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/heyuuu/cube/app"
	"github.com/heyuuu/cube/cmd/util/easycobra"
)

// cmd `server`
var RootCmd = &easycobra.Command{
	Use:   "server",
	Short: `run the server`,
	Args:  cobra.NoArgs,
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		var port int
		cmd.Flags().IntVarP(&port, "port", "p", 8080, "server port")

		return func(args []string) error {
			a := app.Default()
			// 后台监听配置文件变更，热 reload 到各 service（无需重启 server）
			go func() {
				if err := a.WatchConfig(context.Background()); err != nil {
					slog.Warn("config watcher exited", "err", err)
				}
			}()
			return a.Server().Start(fmt.Sprintf(":%d", port))
		}
	},
	Children: []*easycobra.Command{
		openapiCmd,
	},
}

var openapiCmd = &easycobra.Command{
	Use:     "openapi",
	Aliases: []string{"api"},
	Short:   "生成 OpenAPI 3.1 spec 到文件",
	Args:    cobra.NoArgs,
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		var outPath string
		cmd.Flags().StringVarP(&outPath, "out", "o", "openapi.json", "输出文件路径")
		return func(args []string) error {
			return writeOpenAPIFile(outPath)
		}
	},
}

// writeOpenAPIFile 生成 OpenAPI 3.1 spec，格式化后写入 outPath。
func writeOpenAPIFile(outPath string) error {
	// --- 生成 ---
	data, err := app.Default().Server().OpenAPIJSON()
	if err != nil {
		return fmt.Errorf("生成 OpenAPI 失败: %w", err)
	}

	// --- 格式化（2 空格缩进，便于人工检查；HTTP 端点仍用 compact）---
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		return fmt.Errorf("格式化 OpenAPI JSON 失败: %w", err)
	}
	pretty.WriteByte('\n')
	data = pretty.Bytes()

	// --- 准备输出路径 ---
	outPath, err = filepath.Abs(outPath)
	if err != nil {
		return fmt.Errorf("解析输出文件绝对路径失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// --- 写入 ---
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	fmt.Printf("已生成 OpenAPI spec: %s (%d bytes)\n", outPath, len(data))
	return nil
}
