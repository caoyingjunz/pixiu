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

package runtime

import (
	"strings"
	"testing"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
)

// TestNewRejectsInvalidOptions 锁定「配置坏值在首次实际使用运行时（New）时报错」：
// 校验发生在拨号之前，坏值绝不允许进入 containerd/docker 客户端构造。
// 断言错误消息来自 Valid 校验（而非拨号失败）：若有人移除 New 里的 opts.Valid()，
// socket 坏值会落进 containerd.New 拨号，错误文案变为「连接 containerd 失败」且耗时约 10s，
// 本测试即失败。
func TestNewRejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name        string
		opts        config.RuntimeOptions
		wantErrPart string
	}{
		{
			name:        "非法 cri",
			opts:        config.RuntimeOptions{CRI: "podman"},
			wantErrPart: "runtime.cri 取值非法",
		},
		{
			name:        "带协议前缀的 socket",
			opts:        config.RuntimeOptions{CRI: "containerd", Socket: "unix:///run/containerd/containerd.sock"},
			wantErrPart: "runtime.socket 只支持裸 socket 路径",
		},
		{
			name:        "相对路径 socket",
			opts:        config.RuntimeOptions{CRI: "containerd", Socket: "relative.sock"},
			wantErrPart: "runtime.socket 必须是绝对路径",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, err := New(tt.opts)
			if err == nil {
				t.Fatalf("New(%+v) 期望报错，实际返回运行时 %v", tt.opts, rt)
			}
			if rt != nil {
				t.Fatalf("New(%+v) 报错时必须返回 nil 运行时，实际返回 %v", tt.opts, rt)
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("错误消息应包含 %q（来自 Valid 校验），实际: %v", tt.wantErrPart, err)
			}
		})
	}
}

// TestNewContainerdEmptySocketErrors containerd 缺 socket 必须在拨号前直接报错：
// containerd.New 内部 WithBlock 同步拨号（10s 超时），进入拨号会让本测试挂起。
// 错误消息来自拨号前的守卫（构造函数内 address == "" 分支），断言它即可证明未进入拨号。
func TestNewContainerdEmptySocketErrors(t *testing.T) {
	rt, err := New(config.RuntimeOptions{CRI: "containerd"})
	if err == nil {
		t.Fatalf("New(containerd, 空 socket) 期望报错，实际成功（报错前不允许进入拨号）")
	}
	if rt != nil {
		t.Fatalf("报错时必须返回 nil 接口，实际返回 %v", rt)
	}
	if !strings.Contains(err.Error(), "缺少 socket 路径配置") {
		t.Fatalf("错误信息应提示缺少 socket 路径配置，实际: %v", err)
	}
}

// TestNewDockerWithEmptySocket docker SDK 构造无网络 IO，空 socket 表示沿用
// DOCKER_HOST/默认 socket，构造应当成功。
func TestNewDockerWithEmptySocket(t *testing.T) {
	rt, err := New(config.RuntimeOptions{CRI: "docker"})
	if err != nil {
		t.Fatalf("New(docker, 空 socket) 期望成功，实际报错: %v", err)
	}
	defer func() {
		if cerr := rt.Close(); cerr != nil {
			t.Errorf("Close() 报错: %v", cerr)
		}
	}()

	if rt.Kind() != KindDocker {
		t.Fatalf("Kind() = %q，期望 %q", rt.Kind(), KindDocker)
	}
}
