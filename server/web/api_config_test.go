package web

import (
	"testing"

	"cube/config"
)

func TestConfigGet(t *testing.T) {
	env := newTestEnv(t)
	var got config.Config
	decodeData(t, getJSON(t, env.url("/api/config")), &got)

	if got.DataDir != env.ws.Join("data") {
		t.Errorf("dataDir 不符, got %q", got.DataDir)
	}
	if len(got.Project.Scan) != 2 || got.Project.Scan[0].Group != "g1" {
		t.Errorf("project.scan 不符: %+v", got.Project.Scan)
	}
	if len(got.Project.Clone) != 1 || got.Project.Clone[0].RepoHost != "github.com" {
		t.Errorf("project.clone 不符: %+v", got.Project.Clone)
	}
	// config 是原始配置快照：broken opener 原样返回（不做降级过滤，那是 opener.Service 的职责）
	if len(got.Openers) != 2 {
		t.Errorf("openers 应原样返回 2 条（含 broken）, got %d", len(got.Openers))
	}
}
