package config

import (
	"github.com/heyuuu/go-cube/internal/util/pathkit"
	"log"
)

// default

var defaultConf Config

func Default() Config { return defaultConf }

func InitConfig(cfgFile string) error {
	if len(cfgFile) == 0 {
		cfgFile = "~/.go-cube/config.json"
	}

	if IsDebug() {
		log.Println("cfgFile = " + cfgFile)
	}

	cfgFile = pathkit.RealPath(cfgFile)
	return ParseConfigFile(cfgFile, &defaultConf)
}
