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
}
