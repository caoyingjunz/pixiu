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

package updater

import "testing"

func TestCompareVersion(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "v 前缀与无前缀视为相等", a: "v2.0.2", b: "2.0.2", want: 0},
		{name: "高版本大于低版本", a: "v2.0.2", b: "v2.0.1", want: 1},
		{name: "低版本小于高版本", a: "2.0.1", b: "2.0.2", want: -1},
		{name: "无预发布大于有预发布", a: "v2.0.2", b: "v2.0.2-beta.1", want: 1},
		{name: "预发布小于正式版", a: "v2.0.2-beta.1", b: "v2.0.2", want: -1},
		{name: "相同预发布相等", a: "v2.0.2-beta.1", b: "v2.0.2-beta.1", want: 0},
		{name: "预发布序号比较", a: "v2.0.2-beta.2", b: "v2.0.2-beta.1", want: 1},
		{name: "不可解析时回退字符串比较", a: "latest", b: "v2.0.2", want: -1},
		{name: "不可解析与可解析混用", a: "v2.0.2", b: "latest", want: 1},
		{name: "段数不足回退字符串比较", a: "v2.0", b: "v2.0.2", want: -1},
		{name: "数值比较非字典序", a: "1.10.0", b: "1.9.9", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareVersion(tt.a, tt.b); got != tt.want {
				t.Fatalf("compareVersion(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestImageTag(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  string
	}{
		{name: "带 registry 与 tag", image: "registry.example.com/pixiu/pixiu:v2.0.2", want: "v2.0.2"},
		{name: "带 tag 无 registry", image: "pixiu:v2.0.2", want: "v2.0.2"},
		{name: "带 digest 与 tag", image: "registry.example.com/pixiu/pixiu:v2.0.2@sha256:abcdef", want: "v2.0.2"},
		{name: "仅 digest 无 tag", image: "registry.example.com/pixiu/pixiu@sha256:abcdef", want: "pixiu"},
		{name: "无 tag", image: "pixiu", want: "pixiu"},
		{name: "registry 端口不误判为 tag", image: "registry.example.com:5000/pixiu", want: "pixiu"},
		{name: "空镜像", image: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := imageTag(tt.image); got != tt.want {
				t.Fatalf("imageTag(%q) = %q, want %q", tt.image, got, tt.want)
			}
		})
	}
}
