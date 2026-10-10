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

package node

import (
	"testing"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// TestCanProbeWithoutNode 免库内节点（node_id<=0）的连通性检测仅超级管理员可用
func TestCanProbeWithoutNode(t *testing.T) {
	tests := []struct {
		name string
		user *model.User
		want bool
	}{
		{"超级管理员", &model.User{Role: model.RoleRoot}, true},
		{"管理员", &model.User{Role: model.RoleAdmin}, false},
		{"普通用户", &model.User{Role: model.RoleUser}, false},
		{"未登录（nil 用户）", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canProbeWithoutNode(tt.user); got != tt.want {
				t.Errorf("canProbeWithoutNode(%+v) = %v, want %v", tt.user, got, tt.want)
			}
		})
	}
}
