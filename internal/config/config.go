package config

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"strings"
)

func InitConfigFile(cfgFile string) error {
	if len(cfgFile) == 0 {
		cfgFile = "$HOME/.go-cube.yml"
	}

	// 支持 $HOME 前缀
	if strings.HasPrefix(cfgFile, "$HOME/") {
		cfgFile = os.Getenv("HOME") + cfgFile[5:]
	}

	viper.SetConfigFile(cfgFile)

	fmt.Println("cfgFile =", cfgFile)

	return viper.ReadInConfig()
}
