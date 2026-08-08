package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"cube/app"
	"cube/cmd/util/easycobra"
	"cube/cmd/util/tui"
	"cube/config"
	"cube/opener"
	"cube/util/pathkit"
	"cube/version"
)

var RootCmd = &easycobra.Command{
	Use:   "config",
	Short: "show config",
	Run: func(args []string) error {
		fmt.Println("cube " + version.Version)
		fmt.Println("config path: " + config.Path())
		return nil
	},
	Children: []*easycobra.Command{
		configEditCmd,
	},
}

var configEditCmd = &easycobra.Command{
	Use:   "edit",
	Short: "编辑 config 配置文件",
	Run: func(args []string) error {
		cfgFile := filepath.Join(config.Path(), "config.json")

		// config 文件不存在：询问是否创建
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			ok, err := tui.Confirm("config 文件不存在，是否创建？")
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			if err = createDefaultConfigFile(cfgFile); err != nil {
				return err
			}
			fmt.Printf("> 已创建 config: %s\n", pathkit.PrettyPath(cfgFile))
		}

		// 选择 opener 打开 config 文件
		openerService := app.Default().OpenerService()
		openers := openerService.RoleOpeners(opener.RoleOpenFile)
		if len(openers) == 0 {
			return fmt.Errorf("未配置任何可打开文件的 opener，请在 config.json 中为 opener 声明 roles:[\"open-file\"] 后重试")
		}

		opener, err := tui.SelectItem("选择 opener 打开 config", openers, func(o *opener.Opener) string {
			return o.Name()
		})
		if err != nil {
			return err
		}

		if err = opener.Open(cfgFile); err != nil {
			return fmt.Errorf("打开 config 失败: %w", err)
		}
		return nil
	},
}

// createDefaultConfigFile 写入一份最小合法的 config.json（空配置）。
func createDefaultConfigFile(cfgFile string) error {
	data, err := json.MarshalIndent(config.Config{}, "", "  ")
	if err != nil {
		return fmt.Errorf("生成默认 config 失败: %w", err)
	}
	data = append(data, '\n')
	if err = os.WriteFile(cfgFile, data, 0644); err != nil {
		return fmt.Errorf("创建 config 文件失败: %w", err)
	}
	return nil
}
