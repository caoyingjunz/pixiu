/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cicd

import (
	"encoding/json"
	"testing"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func TestRenderPipelineSpecTemplate(t *testing.T) {
	spec := RunSpec{
		EngineName:     "px-1-2",
		Namespace:      "demo",
		SpecMode:       "template",
		Template:       "source-build",
		TimeoutSeconds: 600,
		Config: types.PipelineConfig{
			Repo:     types.PipelineConfigRepo{URL: "https://git.example.com/a/b.git", Ref: "main", Credential: "git-cred"},
			Build:    types.PipelineConfigBuild{Dockerfile: "Dockerfile.app", Context: ".", Args: []string{"GO_VERSION=1.25"}},
			Image:    types.PipelineConfigImage{Registry: "harbor.example.com", Repository: "demo/api", Tag: "v1", Credential: "reg-cred"},
			Advanced: types.PipelineConfigAdvanced{TimeoutSeconds: 600, TLSVerify: true},
		},
	}
	obj, err := renderPipelineSpec(spec)
	if err != nil {
		t.Fatalf("renderPipelineSpec: %v", err)
	}
	if obj["kind"] != "PipelineRun" {
		t.Errorf("kind = %v, want PipelineRun", obj["kind"])
	}
	metadata := obj["metadata"].(map[string]interface{})
	if metadata["name"] != "px-1-2" {
		t.Errorf("name = %v, want px-1-2", metadata["name"])
	}
	prSpec := obj["spec"].(map[string]interface{})
	if _, ok := prSpec["pipelineSpec"]; !ok {
		t.Error("pipelineSpec missing (inline spec expected)")
	}
	timeout := prSpec["timeouts"].(map[string]interface{})
	if timeout["pipeline"] != "600s" {
		t.Errorf("pipeline timeout = %v, want 600s", timeout["pipeline"])
	}
	if _, ok := prSpec["ttlSecondsAfterFinished"]; !ok {
		t.Error("ttlSecondsAfterFinished missing")
	}
	// 校验 workspaces 引用两个凭据 Secret
	ws := prSpec["workspaces"].([]interface{})
	if len(ws) != 2 {
		t.Errorf("workspaces len = %d, want 2", len(ws))
	}
	// 校验两个内联 task
	ps := prSpec["pipelineSpec"].(map[string]interface{})
	tasks := ps["tasks"].([]interface{})
	if len(tasks) != 2 {
		t.Errorf("tasks len = %d, want 2", len(tasks))
	}
}

func TestRenderPipelineSpecRaw(t *testing.T) {
	raw := `{"apiVersion":"tekton.dev/v1","kind":"PipelineRun","metadata":{"name":"custom"}}`
	obj, err := renderPipelineSpec(RunSpec{
		EngineName: "ignored",
		SpecMode:   "raw",
		Template:   "source-build",
		PipelineYaml: raw,
	})
	if err != nil {
		t.Fatalf("renderPipelineSpec raw: %v", err)
	}
	if obj["kind"] != "PipelineRun" {
		t.Errorf("kind = %v", obj["kind"])
	}
	md := obj["metadata"].(map[string]interface{})
	if md["name"] != "custom" {
		t.Errorf("name = %v, want custom (raw should be used verbatim)", md["name"])
	}
}

func TestRenderPipelineSpecUnsupportedTemplate(t *testing.T) {
	_, err := renderPipelineSpec(RunSpec{SpecMode: "template", Template: "unknown"})
	if err == nil {
		t.Error("expected error for unsupported template")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"", "main", "x"}, "main"},
		{[]string{"  ", "  a "}, "a"},
		{[]string{"", ""}, ""},
	}
	for _, c := range cases {
		if got := firstNonEmpty(c.in...); got != c.want {
			t.Errorf("firstNonEmpty(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCICDRunStatusIsTerminal(t *testing.T) {
	terminal := []model.CICDRunStatus{model.CICDRunSucceeded, model.CICDRunFailed, model.CICDRunCancelled, model.CICDRunTimeout}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("%s should be terminal", s)
		}
	}
	for _, s := range []model.CICDRunStatus{model.CICDRunPending, model.CICDRunRunning} {
		if s.IsTerminal() {
			t.Errorf("%s should not be terminal", s)
		}
	}
}

func TestPipelineConfigRoundTrip(t *testing.T) {
	cfg := types.PipelineConfig{
		Repo:  types.PipelineConfigRepo{URL: "u", Ref: "main"},
		Image: types.PipelineConfigImage{Registry: "r", Repository: "repo", Tag: "t"},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := pipelineConfig(string(b))
	if got.Repo.URL != "u" || got.Image.Repository != "repo" {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
}
