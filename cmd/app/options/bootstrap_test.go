/*
Copyright 2024 The Pixiu Authors.

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

package options

import (
	"testing"

	pixiuutil "github.com/caoyingjunz/pixiu/pkg/util"
)

func TestGenerateRandomAdminPassword(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		password, err := generateRandomAdminPassword()
		if err != nil {
			t.Fatalf("generateRandomAdminPassword() error = %v", err)
		}

		if len(password) != 20 {
			t.Errorf("password length = %d, want 20", len(password))
		}
		// 生成结果必须满足仓库强密码校验（大小写字母 + 数字）
		if !pixiuutil.ValidateStrongPassword(password) {
			t.Errorf("generated password %q fails ValidateStrongPassword", password)
		}
		// 多次生成不应重复
		if seen[password] {
			t.Errorf("generated password %q duplicated", password)
		}
		seen[password] = true
	}
}
