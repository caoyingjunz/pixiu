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

func TestSetDefaults(t *testing.T) {
	tests := []struct {
		name       string
		opts       RuntimeOptions
		wantCRI    string
		wantSocket string
	}{
		{
			name:       "containerd 且 socket 空，填入默认 socket",
			opts:       RuntimeOptions{CRI: "containerd"},
			wantCRI:    "containerd",
			wantSocket: defaultContainerdSocket,
		},
		{
			name:       "docker 且 socket 空，保持为空（沿用 DOCKER_HOST/默认 socket）",
			opts:       RuntimeOptions{CRI: "docker"},
			wantCRI:    "docker",
			wantSocket: "",
		},
		{
			name:       "cri 未配置，默认 containerd 并填入默认 socket",
			opts:       RuntimeOptions{},
			wantCRI:    "containerd",
			wantSocket: defaultContainerdSocket,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			opts.SetDefaults()
			if opts.CRI != tt.wantCRI {
				t.Errorf("SetDefaults() CRI = %q, 期望 %q", opts.CRI, tt.wantCRI)
			}
			if opts.Socket != tt.wantSocket {
				t.Errorf("SetDefaults() Socket = %q, 期望 %q", opts.Socket, tt.wantSocket)
			}
		})
	}
}

func TestRuntimeOptionsValid(t *testing.T) {
	tests := []struct {
		name    string
		opts    RuntimeOptions
		wantErr bool
	}{
		{
			name:    "docker 且 socket 空（沿用默认 socket）",
			opts:    RuntimeOptions{CRI: "docker"},
			wantErr: false,
		},
		{
			name:    "containerd 且 socket 空（SetDefaults 后补默认值）",
			opts:    RuntimeOptions{CRI: "containerd"},
			wantErr: false,
		},
		{
			name:    "containerd 且默认 socket 路径",
			opts:    RuntimeOptions{CRI: "containerd", Socket: defaultContainerdSocket},
			wantErr: false,
		},
		{
			name:    "containerd 且相对路径 socket",
			opts:    RuntimeOptions{CRI: "containerd", Socket: "containerd.sock"},
			wantErr: true,
		},
		{
			name:    "docker 且带协议前缀 socket",
			opts:    RuntimeOptions{CRI: "docker", Socket: "unix:///var/run/docker.sock"},
			wantErr: true,
		},
		{
			name:    "非法 cri",
			opts:    RuntimeOptions{CRI: "podman"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			opts.SetDefaults()
			err := opts.Valid()
			if tt.wantErr && err == nil {
				t.Fatalf("Valid() 期望报错，实际通过（%+v）", opts)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Valid() 期望通过，实际报错: %v（%+v）", err, opts)
			}
		})
	}
}

// TestConfigValidDoesNotCheckRuntime 锁定「服务启动链不校验 CRI 配置」的设计：
// 坏值（非法 cri、相对路径 socket）必须放行，延迟到首次实际使用运行时（runtime.New）才报错。
func TestConfigValidDoesNotCheckRuntime(t *testing.T) {
	t.Run("非法 cri + 相对路径 socket 均不阻断启动", func(t *testing.T) {
		cfg := Config{Runtime: RuntimeOptions{CRI: "podman", Socket: "relative.sock"}}
		if err := cfg.Valid(); err != nil {
			t.Fatalf("Config.Valid() 不应校验 runtime 配置，却返回错误: %v", err)
		}
	})

	t.Run("Runtime 全零值不阻断启动", func(t *testing.T) {
		cfg := Config{}
		if err := cfg.Valid(); err != nil {
			t.Fatalf("Config.Valid() 对零值 Runtime 返回错误: %v", err)
		}
	})
}
