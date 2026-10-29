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

package planrender

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	pixiutpl "github.com/caoyingjunz/pixiu/template"
)

func TestRenderExpand(t *testing.T) {
	dir := t.TempDir()
	nodes := []types.PlanNode{
		{
			Name: "worker-1",
			Ip:   "10.0.0.11",
			Role: []string{model.NodeRole},
			CRI:  model.ContainerdCRI,
			Auth: types.PlanNodeAuth{
				Type:     types.PasswordAuth,
				Password: &types.PasswordSpec{User: "root", Password: "pass"},
			},
		},
		{
			Name: "worker-2",
			Ip:   "10.0.0.12",
			Role: []string{model.NodeRole},
			CRI:  model.ContainerdCRI,
			Auth: types.PlanNodeAuth{
				Type: types.KeyAuth,
				Key:  &types.KeySpec{Data: "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----\n"},
			},
		},
	}
	globals := pixiutpl.ExpandGlobals{
		KubeRelease:      "1.28.2",
		APIServer:        "10.0.0.10:6443",
		Token:            "abcdef.0123456789abcdef",
		CAHash:           "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ImageRepository:  "registry.k8s.io",
		NetworkInterface: "eth0",
	}
	customRepo := &types.CustomRepo{Enable: true, Content: "[pixiu]\nname=pixiu\n"}

	if err := RenderExpand(dir, nodes, globals, customRepo); err != nil {
		t.Fatalf("RenderExpand: %v", err)
	}

	multinode := readFile(t, filepath.Join(dir, "multinode"))
	if !strings.Contains(multinode, "[containerd-node]") {
		t.Fatalf("multinode missing containerd-node group:\n%s", multinode)
	}
	if !strings.Contains(multinode, "worker-1") || !strings.Contains(multinode, "worker-2") {
		t.Fatalf("multinode missing target nodes:\n%s", multinode)
	}
	if !strings.Contains(multinode, "[kube-master:children]") {
		t.Fatalf("multinode missing group skeleton:\n%s", multinode)
	}

	hosts := readFile(t, filepath.Join(dir, "hosts"))
	if !strings.Contains(hosts, "10.0.0.11  worker-1") {
		t.Fatalf("hosts missing worker-1:\n%s", hosts)
	}

	globalsFile := readFile(t, filepath.Join(dir, "globals.yml"))
	for _, want := range []string{
		`kube_release: 1.28.2`,
		`kube_apiserver_address: "10.0.0.10:6443"`,
		`kube_admin_token: "abcdef.0123456789abcdef"`,
		`kube_admin_token_ca_cert_hash: "0123456789abcdef`,
		`network_interface: "eth0"`,
	} {
		if !strings.Contains(globalsFile, want) {
			t.Fatalf("globals.yml missing %q:\n%s", want, globalsFile)
		}
	}

	keyFile := filepath.Join(dir, "ssh", "worker-2", "id_rsa")
	if _, err := os.Stat(keyFile); err != nil {
		t.Fatalf("expected ssh key file: %v", err)
	}

	repoFile := readFile(t, filepath.Join(dir, "pixiu"))
	if !strings.Contains(repoFile, "[pixiu]") {
		t.Fatalf("custom repo file not rendered: %s", repoFile)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
