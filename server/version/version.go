// Package version 暴露构建期信息。默认值用于 dev；正式构建用 ldflags 注入。
// e.g. go build -ldflags "-X 'github.com/heyuuu/cube/version.Version=v3.0.0'"

package version

var (
	Version   = "v3.0.0-dev" // -ldflags 注入，默认 dev 标记
	Commit    = "unknown"
	BuildTime = "unknown"
)
