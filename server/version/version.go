// Package version 暴露构建期信息。默认值用于 dev；正式构建用 ldflags 注入。
// e.g. go build -ldflags "-X 'cube/version.version=v3.0.0'"

package version

import (
	"fmt"
)

var (
	version   = "dev" // -ldflags 注入，默认 dev 标记
	commit    = "unknown"
	buildTime = "unknown"
)

// IsDev 判断当前是否在开发环境
func IsDev() bool {
	return version == "dev"
}

func Version() string {
	return version
}

func VersionInfo() string {
	if IsDev() {
		return version
	}
	return fmt.Sprintf("%s (%s %s)", version, commit, buildTime)
}
