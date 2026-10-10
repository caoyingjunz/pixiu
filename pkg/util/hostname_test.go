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

package util

import (
	"strings"
	"testing"
)

func TestIsValidHostname(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"单字符数字", "0", true},
		{"普通主机名", "node-1", true},
		{"中划线在中间", "a-b-c", true},
		{"63 位边界（合法上限）", strings.Repeat("a", 63), true},
		{"64 位超长", strings.Repeat("a", 64), false},
		{"大写字母", "Node1", false},
		{"下划线", "node_1", false},
		{"开头中划线", "-node", false},
		{"结尾中划线", "node-", false},
		{"空字符串", "", false},
		{"含空格", "node 1", false},
		{"前导空格", " node", false},
		{"含点（FQDN）", "node.example.com", false},
		{"inventory 注入样例", `x ansible_ssh_common_args='-o ProxyCommand="sh /configs/pixiu"'`, false},
		{"路径穿越", "../../evil", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidHostname(tt.input); got != tt.want {
				t.Errorf("IsValidHostname(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
