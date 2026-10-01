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

package config

import "testing"

func TestRuntimeOptionsSetDefaultsAndValid(t *testing.T) {
	cases := []struct {
		name          string
		opts          RuntimeOptions
		wantSocket    string
		wantCRI       string // 非空时校验 SetDefaults 后的 CRI
		wantNamespace string // 非空时校验 SetDefaults 后的 Namespace
		wantErr       bool
	}{
		{
			name:          "空配置补全 containerd 默认值",
			opts:          RuntimeOptions{},
			wantSocket:    defaultContainerdSocket,
			wantCRI:       "containerd",
			wantNamespace: "default",
		},
		{
			name:       "docker 不填 socket 默认值",
			opts:       RuntimeOptions{CRI: "docker"},
			wantSocket: "",
		},
		{
			name:       "docker 显式 socket 路径",
			opts:       RuntimeOptions{CRI: "docker", Socket: "/var/run/docker.sock"},
			wantSocket: "/var/run/docker.sock",
		},
		{
			name:       "socket 带协议前缀报错",
			opts:       RuntimeOptions{CRI: "containerd", Socket: "unix:///run/containerd/containerd.sock"},
			wantSocket: "unix:///run/containerd/containerd.sock",
			wantErr:    true,
		},
		{
			name:       "socket 相对路径报错",
			opts:       RuntimeOptions{CRI: "containerd", Socket: "relative/sock"},
			wantSocket: "relative/sock",
			wantErr:    true,
		},
		{
			name:       "containerd 禁止 k8s.io 命名空间",
			opts:       RuntimeOptions{CRI: "containerd", Containerd: ContainerdRuntimeOptions{Namespace: "k8s.io"}},
			wantSocket: defaultContainerdSocket,
			wantErr:    true,
		},
		{
			name:       "非法 CRI 报错",
			opts:       RuntimeOptions{CRI: "podman"},
			wantSocket: "",
			wantErr:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := tc.opts
			o.SetDefaults()

			if o.Socket != tc.wantSocket {
				t.Fatalf("SetDefaults 后 Socket = %q，期望 %q", o.Socket, tc.wantSocket)
			}
			if tc.wantCRI != "" && string(o.CRI) != tc.wantCRI {
				t.Fatalf("SetDefaults 后 CRI = %q，期望 %q", o.CRI, tc.wantCRI)
			}
			if tc.wantNamespace != "" && o.Containerd.Namespace != tc.wantNamespace {
				t.Fatalf("SetDefaults 后 Namespace = %q，期望 %q", o.Containerd.Namespace, tc.wantNamespace)
			}

			err := o.Valid()
			if tc.wantErr && err == nil {
				t.Fatal("Valid() 期望报错，实际为 nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Valid() 报错: %v", err)
			}
		})
	}
}
