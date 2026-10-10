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
)

func testKeyNode(name, ip string) types.PlanNode {
	return types.PlanNode{
		Name: name,
		Role: []string{model.MasterRole},
		Ip:   ip,
		Auth: types.PlanNodeAuth{
			Type: types.KeyAuth,
			Key:  &types.KeySpec{Data: "FAKE-PRIVATE-KEY"},
		},
	}
}

func testPasswordNode(name, ip string) types.PlanNode {
	return types.PlanNode{
		Name: name,
		Role: []string{model.NodeRole},
		Ip:   ip,
		Auth: types.PlanNodeAuth{
			Type:     types.PasswordAuth,
			Password: &types.PasswordSpec{User: "root", Password: "pw123"},
		},
	}
}

func testPlan(id int64, nodes ...types.PlanNode) *types.Plan {
	return &types.Plan{
		PixiuMeta: types.PixiuMeta{Id: id},
		Config: types.PlanConfig{
			Runtime: types.RuntimeSpec{Runtime: string(model.DockerCRI)},
		},
		Nodes: nodes,
	}
}

// TestRenderRejectsMaliciousNodeName 恶意主机名（inventory 注入 / 路径穿越）必须报错且不落盘
func TestRenderRejectsMaliciousNodeName(t *testing.T) {
	injection := `x ansible_ssh_common_args='-o ProxyCommand="sh /configs/pixiu"'`

	t.Run("inventory 注入（含空格）", func(t *testing.T) {
		workDir := t.TempDir()
		plan := testPlan(1, testPasswordNode(injection, "10.0.0.1"))

		if err := Render(workDir, plan); err == nil {
			t.Fatal("Render() = nil, want error for injected hostname")
		}
		// 校验先于任何写入：plan 目录不应被创建
		if _, err := os.Stat(filepath.Join(workDir, "1")); !os.IsNotExist(err) {
			t.Errorf("plan dir should not exist after rejected render, stat err = %v", err)
		}
	})

	t.Run("路径穿越（../../evil）", func(t *testing.T) {
		workDir := t.TempDir()
		plan := testPlan(1, testKeyNode("../../evil", "10.0.0.1"))

		if err := Render(workDir, plan); err == nil {
			t.Fatal("Render() = nil, want error for traversal hostname")
		}
		if _, err := os.Stat(filepath.Join(workDir, "evil")); !os.IsNotExist(err) {
			t.Errorf("escaped dir should not exist, stat err = %v", err)
		}
	})

	t.Run("writeRSA 直接调用", func(t *testing.T) {
		workDir := t.TempDir()
		auth := types.PlanNodeAuth{Type: types.KeyAuth, Key: &types.KeySpec{Data: "FAKE-PRIVATE-KEY"}}
		if _, err := writeRSA(1, "../../evil", workDir, auth); err == nil {
			t.Fatal("writeRSA() = nil, want error for traversal hostname")
		}
	})
}

// TestRenderRejectsSymlinkedIDRSA 目标 id_rsa 被预植为软链时必须报错（O_NOFOLLOW），且不写入软链目标
func TestRenderRejectsSymlinkedIDRSA(t *testing.T) {
	workDir := t.TempDir()
	victimDir := t.TempDir()
	victim := filepath.Join(victimDir, "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o600); err != nil {
		t.Fatalf("prepare victim file: %v", err)
	}

	planDir := filepath.Join(workDir, "1")
	rsaDir := filepath.Join(planDir, "ssh", "n1")
	if err := os.MkdirAll(rsaDir, 0o755); err != nil {
		t.Fatalf("prepare rsa dir: %v", err)
	}
	if err := os.Symlink(victim, filepath.Join(rsaDir, "id_rsa")); err != nil {
		t.Fatalf("plant symlink: %v", err)
	}

	plan := testPlan(1, testKeyNode("n1", "10.0.0.1"))
	if err := Render(workDir, plan); err == nil {
		t.Fatal("Render() = nil, want error for symlinked id_rsa")
	}

	data, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("read victim: %v", err)
	}
	if string(data) != "keep" {
		t.Errorf("victim file overwritten via symlink: %q", data)
	}
}

// TestRenderRejectsSymlinkedSSHDir ssh 目录被替换为软链时必须报错，且不把密钥写到软链目标
func TestRenderRejectsSymlinkedSSHDir(t *testing.T) {
	workDir := t.TempDir()
	elsewhere := t.TempDir()

	planDir := filepath.Join(workDir, "1")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("prepare plan dir: %v", err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(planDir, "ssh")); err != nil {
		t.Fatalf("plant symlink: %v", err)
	}

	plan := testPlan(1, testKeyNode("n1", "10.0.0.1"))
	if err := Render(workDir, plan); err == nil {
		t.Fatal("Render() = nil, want error for symlinked ssh dir")
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "n1", "id_rsa")); !os.IsNotExist(err) {
		t.Errorf("id_rsa should not be written through symlinked ssh dir, stat err = %v", err)
	}
}

// TestRenderRejectsSymlinkedPlanDir plan 目录本身被预植为软链时必须报错
func TestRenderRejectsSymlinkedPlanDir(t *testing.T) {
	workDir := t.TempDir()
	elsewhere := t.TempDir()
	if err := os.Symlink(elsewhere, filepath.Join(workDir, "1")); err != nil {
		t.Fatalf("plant symlink: %v", err)
	}

	plan := testPlan(1, testPasswordNode("n1", "10.0.0.1"))
	if err := Render(workDir, plan); err == nil {
		t.Fatal("Render() = nil, want error for symlinked plan dir")
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "hosts")); !os.IsNotExist(err) {
		t.Errorf("hosts should not be written through symlinked plan dir, stat err = %v", err)
	}
}

// TestRenderNormal 正常计划：文件落盘、内容正确、权限 0600（含既有宽松权限被收紧）
func TestRenderNormal(t *testing.T) {
	workDir := t.TempDir()
	plan := testPlan(42,
		testPasswordNode("master-1", "10.0.0.1"),
		testKeyNode("node-1", "10.0.0.2"),
	)
	plan.Config.Kubernetes = types.KubernetesSpec{KubernetesVersion: "v1.28.4"}
	plan.Config.Network = types.NetworkSpec{PodNetwork: "10.244.0.0/16", ServiceNetwork: "10.96.0.0/12"}
	plan.Config.Component.CustomRepo = &types.CustomRepo{Enable: true, Content: "[pixiu]\nname=pixiu\n"}

	planDir := filepath.Join(workDir, "42")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("prepare plan dir: %v", err)
	}
	// 预植宽松权限的既有文件，验证渲染后权限被收紧为 0600
	if err := os.WriteFile(filepath.Join(planDir, "globals.yml"), []byte("old"), 0o644); err != nil {
		t.Fatalf("prepare legacy globals.yml: %v", err)
	}

	if err := Render(workDir, plan); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// multinode：两类认证行均按模板渲染
	multinode, err := os.ReadFile(filepath.Join(planDir, "multinode"))
	if err != nil {
		t.Fatalf("read multinode: %v", err)
	}
	for _, want := range []string{
		"master-1 ansible_ssh_user=root ansible_ssh_pass=pw123",
		"node-1 ansible_ssh_user=root ansible_ssh_private_key_file=/configs/ssh/node-1/id_rsa",
	} {
		if !strings.Contains(string(multinode), want) {
			t.Errorf("multinode missing %q, got:\n%s", want, multinode)
		}
	}

	// hosts / globals / 自定义源文件
	hosts, err := os.ReadFile(filepath.Join(planDir, "hosts"))
	if err != nil {
		t.Fatalf("read hosts: %v", err)
	}
	if !strings.Contains(string(hosts), "10.0.0.1  master-1") {
		t.Errorf("hosts missing master-1 entry, got:\n%s", hosts)
	}
	globals, err := os.ReadFile(filepath.Join(planDir, "globals.yml"))
	if err != nil {
		t.Fatalf("read globals.yml: %v", err)
	}
	if !strings.Contains(string(globals), "kube_release: v1.28.4") {
		t.Errorf("globals.yml missing kube_release, got:\n%s", globals)
	}
	repo, err := os.ReadFile(filepath.Join(planDir, "pixiu"))
	if err != nil {
		t.Fatalf("read pixiu repo file: %v", err)
	}
	if string(repo) != "[pixiu]\nname=pixiu\n" {
		t.Errorf("pixiu repo content = %q", repo)
	}

	// 密钥落盘、内容正确、权限 0600
	rsa, err := os.ReadFile(filepath.Join(planDir, "ssh", "node-1", "id_rsa"))
	if err != nil {
		t.Fatalf("read id_rsa: %v", err)
	}
	if string(rsa) != "FAKE-PRIVATE-KEY" {
		t.Errorf("id_rsa content = %q", rsa)
	}

	for _, f := range []string{"hosts", "multinode", "globals.yml", "pixiu", filepath.Join("ssh", "node-1", "id_rsa")} {
		info, err := os.Stat(filepath.Join(planDir, f))
		if err != nil {
			t.Fatalf("stat %s: %v", f, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("%s perm = %o, want 600", f, got)
		}
	}
}

// TestWriteFileInDirContainment 直接单测写文件助手的目录 containment
func TestWriteFileInDirContainment(t *testing.T) {
	base := t.TempDir()
	outside := filepath.Join(base, "..", "escape-"+filepath.Base(base))
	if err := writeFileInDir(base, outside, []byte("x")); err == nil {
		t.Fatal("writeFileInDir() = nil, want containment error")
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Errorf("escaped file should not exist, stat err = %v", err)
	}
}
