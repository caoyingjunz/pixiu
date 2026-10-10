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

package plan

import (
	"strings"
	"testing"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func marshalAuth(t *testing.T, auth types.PlanNodeAuth) string {
	t.Helper()
	s, err := auth.Marshal()
	if err != nil {
		t.Fatalf("marshal auth: %v", err)
	}
	return s
}

func passwordAuth() types.PlanNodeAuth {
	return types.PlanNodeAuth{
		Type:     types.PasswordAuth,
		Password: &types.PasswordSpec{User: "root", Password: "p@ssw0rd"},
	}
}

func newValidTaskData(t *testing.T) TaskData {
	t.Helper()
	ks := types.KubernetesSpec{KubernetesVersion: "v1.28.4"}
	k8sJSON, err := ks.Marshal()
	if err != nil {
		t.Fatalf("marshal kubernetes spec: %v", err)
	}
	ns := types.NetworkSpec{PodNetwork: "10.244.0.0/16", ServiceNetwork: "10.96.0.0/12"}
	netJSON, err := ns.Marshal()
	if err != nil {
		t.Fatalf("marshal network spec: %v", err)
	}

	return TaskData{
		PlanId: 1,
		Plan:   &model.Plan{Name: "demo"},
		Config: &model.Config{PlanId: 1, Kubernetes: k8sJSON, Network: netJSON},
		Nodes: []model.Node{
			{Name: "master-1", Ip: "10.0.0.1", Role: model.MasterRole, Auth: marshalAuth(t, passwordAuth())},
		},
	}
}

func setNodeAuth(t *testing.T, d *TaskData, auth types.PlanNodeAuth) {
	t.Helper()
	d.Nodes[0].Auth = marshalAuth(t, auth)
}

func setNetwork(t *testing.T, d *TaskData, ns types.NetworkSpec) {
	t.Helper()
	netJSON, err := ns.Marshal()
	if err != nil {
		t.Fatalf("marshal network spec: %v", err)
	}
	d.Config.Network = netJSON
}

func TestTaskDataValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(t *testing.T, d *TaskData)
		wantErr string // 期望错误信息包含的子串，空串表示应校验通过
	}{
		{
			name:   "合法组合（password）",
			mutate: func(*testing.T, *TaskData) {},
		},
		{
			name: "合法组合（key）",
			mutate: func(t *testing.T, d *TaskData) {
				setNodeAuth(t, d, types.PlanNodeAuth{Type: types.KeyAuth, Key: &types.KeySpec{Data: "FAKE-KEY"}})
			},
		},
		{
			name: "合法组合（非默认端口）",
			mutate: func(t *testing.T, d *TaskData) {
				auth := passwordAuth()
				auth.Port = 2222
				setNodeAuth(t, d, auth)
			},
		},
		{
			name:    "plan 为空",
			mutate:  func(_ *testing.T, d *TaskData) { d.Plan = nil },
			wantErr: "部署计划数据异常",
		},
		{
			name:    "planId 非法",
			mutate:  func(_ *testing.T, d *TaskData) { d.PlanId = 0 },
			wantErr: "部署计划数据异常",
		},
		{
			name:    "config 为空",
			mutate:  func(_ *testing.T, d *TaskData) { d.Config = nil },
			wantErr: "配置缺失",
		},
		{
			name:    "节点为空",
			mutate:  func(_ *testing.T, d *TaskData) { d.Nodes = nil },
			wantErr: "未包含任何节点",
		},
		{
			name: "主机名含注入字符",
			mutate: func(_ *testing.T, d *TaskData) {
				d.Nodes[0].Name = `x ansible_ssh_common_args='-o ProxyCommand="sh /configs/pixiu"'`
			},
			wantErr: "主机名",
		},
		{
			name:    "主机名大写",
			mutate:  func(_ *testing.T, d *TaskData) { d.Nodes[0].Name = "Master-1" },
			wantErr: "主机名",
		},
		{
			name:    "主机名以中划线结尾",
			mutate:  func(_ *testing.T, d *TaskData) { d.Nodes[0].Name = "master-" },
			wantErr: "主机名",
		},
		{
			name:    "IP 非法",
			mutate:  func(_ *testing.T, d *TaskData) { d.Nodes[0].Ip = "10.0.0" },
			wantErr: "IP 地址",
		},
		{
			name:    "IP 为空",
			mutate:  func(_ *testing.T, d *TaskData) { d.Nodes[0].Ip = "" },
			wantErr: "IP 地址",
		},
		{
			name:    "auth 无法解析",
			mutate:  func(_ *testing.T, d *TaskData) { d.Nodes[0].Auth = "" },
			wantErr: "认证信息无法解析",
		},
		{
			name: "密码为空",
			mutate: func(t *testing.T, d *TaskData) {
				setNodeAuth(t, d, types.PlanNodeAuth{Type: types.PasswordAuth, Password: &types.PasswordSpec{User: "root"}})
			},
			wantErr: "缺少密码认证信息",
		},
		{
			name: "密码含空格",
			mutate: func(t *testing.T, d *TaskData) {
				setNodeAuth(t, d, types.PlanNodeAuth{Type: types.PasswordAuth, Password: &types.PasswordSpec{User: "root", Password: "pw 123"}})
			},
			wantErr: "密码包含不支持字符",
		},
		{
			name: "密码含引号",
			mutate: func(t *testing.T, d *TaskData) {
				setNodeAuth(t, d, types.PlanNodeAuth{Type: types.PasswordAuth, Password: &types.PasswordSpec{User: "root", Password: `pw"1'2`}})
			},
			wantErr: "密码包含不支持字符",
		},
		{
			name: "SSH 用户非法",
			mutate: func(t *testing.T, d *TaskData) {
				setNodeAuth(t, d, types.PlanNodeAuth{Type: types.PasswordAuth, Password: &types.PasswordSpec{User: "Root", Password: "pw123"}})
			},
			wantErr: "SSH 用户",
		},
		{
			name: "SSH 端口越界",
			mutate: func(t *testing.T, d *TaskData) {
				auth := passwordAuth()
				auth.Port = 70000
				setNodeAuth(t, d, auth)
			},
			wantErr: "SSH 端口",
		},
		{
			name: "认证方式不支持",
			mutate: func(t *testing.T, d *TaskData) {
				setNodeAuth(t, d, types.PlanNodeAuth{Type: types.NoneAuth, Password: &types.PasswordSpec{User: "root", Password: "pw123"}})
			},
			wantErr: "认证方式",
		},
		{
			name: "key 认证缺私钥",
			mutate: func(t *testing.T, d *TaskData) {
				setNodeAuth(t, d, types.PlanNodeAuth{Type: types.KeyAuth, Key: &types.KeySpec{}})
			},
			wantErr: "缺少密钥认证信息",
		},
		{
			name: "K8s 版本为空",
			mutate: func(t *testing.T, d *TaskData) {
				ks := types.KubernetesSpec{}
				k8sJSON, err := ks.Marshal()
				if err != nil {
					t.Fatalf("marshal kubernetes spec: %v", err)
				}
				d.Config.Kubernetes = k8sJSON
			},
			wantErr: "K8s 版本",
		},
		{
			name: "K8s 版本含注入字符",
			mutate: func(t *testing.T, d *TaskData) {
				ks := types.KubernetesSpec{KubernetesVersion: "v1.28.4\nkube_vip_address: evil"}
				k8sJSON, err := ks.Marshal()
				if err != nil {
					t.Fatalf("marshal kubernetes spec: %v", err)
				}
				d.Config.Kubernetes = k8sJSON
			},
			wantErr: "K8s 版本",
		},
		{
			name: "容器子网非法 CIDR",
			mutate: func(t *testing.T, d *TaskData) {
				setNetwork(t, d, types.NetworkSpec{PodNetwork: "10.244.0.0", ServiceNetwork: "10.96.0.0/12"})
			},
			wantErr: "容器子网",
		},
		{
			name: "Service 段非法 CIDR",
			mutate: func(t *testing.T, d *TaskData) {
				setNetwork(t, d, types.NetworkSpec{PodNetwork: "10.244.0.0/16", ServiceNetwork: "10.96.0.0/99"})
			},
			wantErr: "Service IP 段",
		},
		{
			name: "多节点其中第二个非法",
			mutate: func(t *testing.T, d *TaskData) {
				d.Nodes = append(d.Nodes, model.Node{Name: "bad name", Ip: "10.0.0.2", Role: model.NodeRole, Auth: marshalAuth(t, passwordAuth())})
			},
			wantErr: "主机名",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := newValidTaskData(t)
			tt.mutate(t, &data)

			err := data.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validate() = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("validate() = %q, want error containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}
