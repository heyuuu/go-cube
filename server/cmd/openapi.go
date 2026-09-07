package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"cube/app"
)

func newOpenapiCmd(a *app.App) *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:     "openapi",
		Aliases: []string{"api"},
		Short:   "生成 OpenAPI 3.1 spec 到文件",
		Long: `导出 cube server HTTP API 的 OpenAPI 3.1 spec 到文件，
用于人工检查或给对接工具使用。

生成不依赖 server 运行，直接来自装配好的路由；输出为
2 空格缩进的格式化 JSON，默认写 openapi.json，
-o 可指定输出路径（所在目录不存在会自动创建）。`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeOpenAPIFile(a, outPath)
		},
	}

	cmd.Flags().StringVarP(&outPath, "out", "o", "openapi.json", "输出文件路径")
	return cmd
}

// writeOpenAPIFile 生成 OpenAPI 3.1 spec，格式化后写入 outPath。
func writeOpenAPIFile(a *app.App, outPath string) error {
	data, err := a.OpenAPIJSON()
	if err != nil {
		return fmt.Errorf("生成 OpenAPI 失败: %w", err)
	}

	// 2 空格缩进便于人工检查（HTTP 端点仍用 compact）
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		return fmt.Errorf("格式化 OpenAPI JSON 失败: %w", err)
	}
	pretty.WriteByte('\n')
	data = pretty.Bytes()

	outPath, err = filepath.Abs(outPath)
	if err != nil {
		return fmt.Errorf("解析输出文件绝对路径失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	fmt.Printf("已生成 OpenAPI spec: %s (%d bytes)\n", outPath, len(data))
	return nil
}
